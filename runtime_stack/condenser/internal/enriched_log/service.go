package enrichedlog

import (
	"bufio"
	"bytes"
	"condenser/internal/store/csm"
	"condenser/internal/store/ipam"
	"condenser/internal/utils"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

func NewResolver(ipamHandler ipam.IpamHandler, csmHandler csm.CsmHandler) *Resolver {
	resolver := &Resolver{
		ResolveMap:  map[string]ContainerMeta{},
		ipamHandler: ipamHandler,
		csmHandler:  csmHandler,
	}
	pool, _ := ipamHandler.GetPoolList()
	for _, p := range pool {
		for addr, info := range p.Allocations {
			if _, ok := resolver.ResolveMap[addr]; !ok {
				containerName, _ := csmHandler.GetContainerNameById(info.ContainerId)
				spiffeId, _ := csmHandler.GetSpiffeById(info.ContainerId)
				resolver.ResolveMap[addr] = ContainerMeta{
					ContainerId:   info.ContainerId,
					ContainerName: containerName,
					Ipv4:          addr,
					Veth:          info.Interface,
					SpiffeId:      spiffeId,
				}
			}
		}
	}
	return resolver
}

type Resolver struct {
	ResolveMap  map[string]ContainerMeta
	ipamHandler ipam.IpamHandler
	csmHandler  csm.CsmHandler
}

func (r *Resolver) Refresh() {
	r.ResolveMap = map[string]ContainerMeta{}
	pool, _ := r.ipamHandler.GetPoolList()
	for _, p := range pool {
		for addr, info := range p.Allocations {
			if _, ok := r.ResolveMap[addr]; !ok {
				containerName, err := r.csmHandler.GetContainerNameById(info.ContainerId)
				if err != nil {
					continue
				}
				spiffeId, _ := r.csmHandler.GetSpiffeById(info.ContainerId)
				r.ResolveMap[addr] = ContainerMeta{
					ContainerId:   info.ContainerId,
					ContainerName: containerName,
					Ipv4:          addr,
					Veth:          info.Interface,
					SpiffeId:      spiffeId,
				}
			}
		}
	}
}

func (r *Resolver) Watch(ctx context.Context) error {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	dir := filepath.Dir(utils.CsmStorePath)
	base := filepath.Base(utils.CsmStorePath)

	if err := w.Add(dir); err != nil {
		return err
	}

	var pending atomic.Bool
	trigger := func() {
		if pending.CompareAndSwap(false, true) {
			go func() {
				time.Sleep(50 * time.Millisecond)
				r.Refresh()
				pending.Store(false)
			}()
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev := <-w.Events:
			if filepath.Base(ev.Name) != base {
				continue
			}
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
				trigger()
			}
		case <-w.Errors:
		}
	}
}

func NewEnrichedLogHandler() *EnrichedLogHandler {
	return &EnrichedLogHandler{
		ipamHandler: ipam.NewIpamManager(ipam.NewIpamStore(utils.IpamStorePath)),
	}
}

type EnrichedLogHandler struct {
	ipamHandler ipam.IpamHandler
}

func (h *EnrichedLogHandler) EnrichedLogger() {
	runtimeSubnet, err := h.ipamHandler.GetRuntimeSubnet()
	if err != nil {
		runtimeSubnet = "10.166.0.0/16"
	}
	_, subnet, err := net.ParseCIDR(runtimeSubnet)
	if err != nil {
		panic(err)
	}

	resolver := NewResolver(
		ipam.NewIpamManager(ipam.NewIpamStore(utils.IpamStorePath)),
		csm.NewCsmManager(csm.NewCsmStore(utils.CsmStorePath)),
	)

	// start resolver watch
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if err := resolver.Watch(ctx); err != nil {
			log.Printf("watch stopped: %v", err)
		}
	}()

	enricher := &Enricher{
		RuntimeSubnet: subnet,
		OutPath:       utils.EnrichedLogPath,
		Resolver:      resolver,
	}
	if err := enricher.OpenOutput(); err != nil {
		panic(err)
	}
	defer enricher.CloseOutput()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tailer := &Tailer{
		Path:         utils.UlogPath,
		PollInterval: 500 * time.Millisecond,
	}

	if err := tailer.Follow(ctx, enricher.HandleRawLine); err != nil && !errors.Is(err, context.Canceled) {
		panic(err)
	}
}

// ulogd2-json raw record
type RawRecord map[string]any

func (rr RawRecord) str(key string) string {
	v, ok := rr[key]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		// json numbers
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%f", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

func (rr RawRecord) int(key string) (int, bool) {
	v, ok := rr[key]
	if !ok || v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return int(t), true
	case int:
		return t, true
	case json.Number:
		i, err := t.Int64()
		return int(i), err == nil
	default:
		return 0, false
	}
}

// enriched log output
type Tailer struct {
	Path         string
	PollInterval time.Duration
}

func (t *Tailer) Follow(ctx context.Context, handleLine func([]byte)) error {
	var (
		f      *os.File
		rd     *bufio.Reader
		inode  uint64
		offset int64
	)

	openFile := func() error {
		if f != nil {
			_ = f.Close()
			f = nil
		}
		file, err := os.Open(t.Path)
		if err != nil {
			return err
		}
		st, err := file.Stat()
		if err != nil {
			_ = file.Close()
			return err
		}
		inode = getInode(st)

		off, err := file.Seek(0, io.SeekEnd)
		if err != nil {
			_ = file.Close()
			return err
		}
		offset = off
		f = file
		rd = bufio.NewReaderSize(f, 256*1024)
		return nil
	}

	// first open
	for {
		if err := openFile(); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(t.PollInterval):
					continue
				}
			}
			return err
		}
		break
	}

	ticker := time.NewTicker(t.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if f != nil {
				_ = f.Close()
			}
			return ctx.Err()
		default:
		}

		// read lines
		for {
			line, err := rd.ReadBytes('\n')
			if err == nil {
				offset += int64(len(line))
				handleLine(bytes.TrimSpace(line))
				continue
			}
			if errors.Is(err, io.EOF) {
				break
			}
			break
		}

		// detect rotation / truncate
		select {
		case <-ctx.Done():
			if f != nil {
				_ = f.Close()
			}
			return ctx.Err()
		case <-ticker.C:
			st, err := os.Stat(t.Path)
			if err != nil {
				continue
			}
			curInode := getInode(st)
			curSize := st.Size()

			// rotate=inode change
			if curInode != inode {
				_ = openFile()
				continue
			}
			// truncate
			if curSize < offset {
				_ = openFile()
				continue
			}
		}
	}
}

func getInode(fi os.FileInfo) uint64 {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok || st == nil {
		return 0
	}
	return st.Ino
}

// enrichment
type Enricher struct {
	RuntimeSubnet *net.IPNet
	OutPath       string
	Resolver      *Resolver

	muOut sync.Mutex
	out   *os.File
	bw    *bufio.Writer
	size  int64
}

func (e *Enricher) OpenOutput() error {
	if err := os.MkdirAll(filepath.Dir(e.OutPath), 0750); err != nil {
		return err
	}
	f, err := os.OpenFile(e.OutPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return err
	}
	var size int64
	if st, err := f.Stat(); err == nil {
		size = st.Size()
	}
	e.out = f
	e.bw = bufio.NewWriterSize(f, 256*1024)
	e.size = size
	return nil
}

func (e *Enricher) CloseOutput() {
	e.muOut.Lock()
	defer e.muOut.Unlock()
	if e.bw != nil {
		_ = e.bw.Flush()
	}
	if e.out != nil {
		_ = e.out.Close()
	}
}

func (e *Enricher) HandleRawLine(line []byte) {
	if len(line) == 0 {
		return
	}
	var rr RawRecord
	if err := json.Unmarshal(line, &rr); err != nil {
		return
	}

	en := e.enrich(rr, line)
	b, err := json.Marshal(en)
	if err != nil {
		return
	}

	e.muOut.Lock()
	defer e.muOut.Unlock()
	if err := e.rotateIfNeeded(int64(len(b) + 1)); err != nil {
		return
	}
	_, _ = e.bw.Write(b)
	_, _ = e.bw.WriteString("\n")
	_ = e.bw.Flush()
	e.size += int64(len(b) + 1)
}

func (e *Enricher) enrich(rr RawRecord, rawLine []byte) Enriched {
	now := time.Now().Format(time.RFC3339Nano)

	genTs := rr.str("timestamp")
	prefix := rr.str("oob.prefix")
	srcIp := rr.str("src_ip")
	dstIp := rr.str("dest_ip")
	srcPort, _ := rr.int("src_port")
	destPort, _ := rr.int("dest_port")
	ipProto, _ := rr.int("ip.protocol")
	proto := ipProtoToName(ipProto, rr)

	rawHash := sha256.Sum256(rawLine)
	kind, verdict, policyId := classify(prefix)
	var policy Policy
	if policyId == "predefined" {
		policy.Source = "predefined"
	} else {
		policy.Source = "user"
		policy.Id = policyId
	}

	src := Endpoint{
		Kind: "unknown",
		Ip:   srcIp,
		Port: srcPort,
	}
	dst := Endpoint{
		Kind: "unknown",
		Ip:   dstIp,
		Port: destPort,
	}

	srcIsContainer := e.isInRuntimeSubnet(srcIp)
	dstIsContainer := e.isInRuntimeSubnet(dstIp)

	if srcIp != "" {
		if srcIsContainer {
			src.Kind = "container"
			containerMeta, ok := e.Resolver.ResolveMap[srcIp]
			if !ok {
				src.Kind = "container_unresolved"
			}
			src.ContainerId, src.ContainerName, src.Veth, src.SpiffeId = containerMeta.ContainerId, containerMeta.ContainerName, containerMeta.Veth, containerMeta.SpiffeId
		} else {
			src.Kind = "external"
		}
	}
	if dstIp != "" {
		if dstIsContainer {
			dst.Kind = "container"
			containerMeta, ok := e.Resolver.ResolveMap[dstIp]
			if !ok {
				dst.Kind = "container_unresolved"
			}
			dst.ContainerId, dst.ContainerName, dst.Veth, dst.SpiffeId = containerMeta.ContainerId, containerMeta.ContainerName, containerMeta.Veth, containerMeta.SpiffeId
		} else {
			dst.Kind = "external"
		}
	}

	out := Enriched{
		GeneratedTS: genTs,
		ReceivedTS:  now,
		EventType:   "log.netflow",
		Policy:      policy,
		Kind:        kind,
		Verdict:     verdict,
		Proto:       proto,
		Src:         src,
		Dst:         dst,
		RuleHint:    prefix,
		RawHash:     hex.EncodeToString(rawHash[:]),
	}

	// ICMP
	if proto == "ICMP" || proto == "ICMPV6" {
		icmp := map[string]int{}
		if v, ok := rr.int("icmp.type"); ok {
			icmp["type"] = v
		}
		if v, ok := rr.int("icmp.code"); ok {
			icmp["code"] = v
		}
		if v, ok := rr.int("icmp.ecchoseq"); ok {
			icmp["seq"] = v
		}
		if len(icmp) > 0 {
			out.ICMP = icmp
		}
	}

	// unresolved
	var reasons []string
	if srcIsContainer && src.ContainerId == "" {
		reasons = append(reasons, "src ip not mapped")
	}
	if dstIsContainer && dst.ContainerId == "" {
		reasons = append(reasons, "dst ip not mapped")
	}
	if len(reasons) > 0 {
		out.Unresolved = true
		out.Reason = strings.Join(reasons, "; ")
	}

	return out
}

const (
	enrichedMaxBytes   = 50 * 1024 * 1024
	enrichedMaxBackups = 5
)

func (e *Enricher) rotateIfNeeded(nextBytes int64) error {
	if enrichedMaxBytes <= 0 {
		return nil
	}
	if e.size+nextBytes <= enrichedMaxBytes {
		return nil
	}
	return e.rotate()
}

func (e *Enricher) rotate() error {
	if e.bw != nil {
		_ = e.bw.Flush()
	}
	if e.out != nil {
		_ = e.out.Close()
	}

	if enrichedMaxBackups > 0 {
		last := e.OutPath + "." + strconv.Itoa(enrichedMaxBackups)
		_ = os.Remove(last)
		for i := enrichedMaxBackups - 1; i >= 1; i-- {
			src := e.OutPath + "." + strconv.Itoa(i)
			dst := e.OutPath + "." + strconv.Itoa(i+1)
			if _, err := os.Stat(src); err == nil {
				_ = os.Rename(src, dst)
			}
		}
		_ = os.Rename(e.OutPath, e.OutPath+".1")
	} else {
		_ = os.Remove(e.OutPath)
	}

	f, err := os.OpenFile(e.OutPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		return err
	}
	e.out = f
	e.bw = bufio.NewWriterSize(f, 256*1024)
	e.size = 0
	return nil
}

func (e *Enricher) isInRuntimeSubnet(ipStr string) bool {
	ipStr = strings.TrimSpace(ipStr)
	if ipStr == "" {
		return false
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	if v4 := ip.To4(); v4 != nil {
		return e.RuntimeSubnet.Contains(v4)
	}
	return false
}

func ipProtoToName(ipProto int, rr RawRecord) string {
	switch ipProto {
	case 6:
		return "TCP"
	case 17:
		return "UDP"
	case 1:
		return "ICMP"
	case 58:
		return "ICMPV6"
	default:
		if ipProto != 0 {
			return fmt.Sprintf("IPPROTO_%d", ipProto)
		}
		return "UNKNOWN"
	}
}

func classify(prefix string) (kind string, verdict string, policyId string) {
	parts := strings.SplitN(prefix, ",", 2)
	id := strings.SplitN(parts[1], "=", 2)[1]

	p := strings.ToUpper(prefix)
	switch {
	case strings.Contains(p, "EW") && strings.Contains(p, "ALLOW"):
		return "east-west", "allow", id
	case strings.Contains(p, "EW") && strings.Contains(p, "DENY"):
		return "east-west", "deny", id
	case strings.Contains(p, "NS") && strings.Contains(p, "ALLOW"):
		return "north-south", "allow", id
	case strings.Contains(p, "NS") && strings.Contains(p, "DENY"):
		return "north-south", "deny", id
	default:
		return "unknown", "unknown", "unknown"
	}
}
