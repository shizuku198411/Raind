package dns

import (
	"condenser/internal/store/csm"
	"condenser/internal/store/ipam"
	"condenser/internal/utils"
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
)

func StartDnsProxy() {
	service := ipam.NewIpamManager(ipam.NewIpamStore(utils.IpamStorePath))
	_, proxyAddr, upstreams, err := service.GetDnsProxyInfo()
	if err != nil {
		log.Fatal(err)
	}
	var (
		listenAddr = proxyAddr + ":1053"
		timeoutMs  = 1500
		cacheKeys  = 10000
	)

	rand.Seed(time.Now().UnixNano())

	upList := []string{}
	for _, u := range upstreams {
		if !strings.Contains(u, ":53") {
			upList = append(upList, u+":53")
		} else {
			upList = append(upList, u)
		}
	}

	var cache *DnsCache
	if cacheKeys > 0 {
		cache = NewDnsCache(cacheKeys)
	} else {
		cache = NewDnsCache(0)
	}

	fwd := NewDnsProxy(upList, time.Duration(timeoutMs)*time.Millisecond, cache)
	if fwd == nil {
		log.Fatal(err)
		return
	}

	mux := dns.NewServeMux()
	mux.HandleFunc(".", fwd.ServeDns)

	udpServer := &dns.Server{Addr: listenAddr, Net: "udp", Handler: mux}
	tcpServer := &dns.Server{Addr: listenAddr, Net: "tcp", Handler: mux}

	go func() {
		log.Printf("[*] dns proxy start udp listen=%s upstreams=%v", listenAddr, upList)
		err := udpServer.ListenAndServe()
		if err != nil {
			log.Fatal(err)
			return
		}
	}()
	go func() {
		log.Printf("[*] dns proxy start tcp listen=%s upstreams=%v", listenAddr, upList)
		err := tcpServer.ListenAndServe()
		if err != nil {
			log.Fatal(err)
			return
		}
	}()
	go func() {
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()
		for range t.C {
			_ = fwd.logger.Flush()
		}
	}()
}

type DnsProxy struct {
	upstreams []string
	timeout   time.Duration
	cache     *DnsCache

	udpClient *dns.Client
	tcpClient *dns.Client

	csmHandler  csm.CsmHandler
	ipamHandler ipam.IpamHandler

	logger *DnsLogger
}

func NewDnsProxy(upstreams []string, timeout time.Duration, cache *DnsCache) *DnsProxy {
	dnsLogger, err := NewDnsLogger(utils.DnsLogPath)
	if err != nil {
		return nil
	}
	return &DnsProxy{
		upstreams: upstreams,
		timeout:   timeout,
		cache:     cache,
		udpClient: &dns.Client{
			Net:     "udp",
			Timeout: timeout,
		},
		tcpClient: &dns.Client{
			Net:     "tcp",
			Timeout: timeout,
		},
		csmHandler:  csm.NewCsmManager(csm.NewCsmStore(utils.CsmStorePath)),
		ipamHandler: ipam.NewIpamManager(ipam.NewIpamStore(utils.IpamStorePath)),

		logger: dnsLogger,
	}
}

func (f *DnsProxy) pickUpstream() (string, error) {
	if len(f.upstreams) == 0 {
		return "", errors.New("no upstream configured")
	}
	return f.upstreams[rand.Intn(len(f.upstreams))], nil
}

func (f *DnsProxy) exchange(ctx context.Context, req *dns.Msg) (*dns.Msg, string, time.Duration, error) {
	up, err := f.pickUpstream()
	if err != nil {
		return nil, "", 0, err
	}

	start := time.Now()
	resp, _, err := f.udpClient.ExchangeContext(ctx, req, up)
	d := time.Since(start)

	// if the udp responce is TC=1, fallback to tcp
	if err == nil && resp != nil && resp.Truncated {
		start2 := time.Now()
		resp2, _, err2 := f.tcpClient.ExchangeContext(ctx, req, up)
		d = time.Since(start2)
		if err2 == nil && resp2 != nil {
			return resp2, up + " (tcp)", d, nil
		}
		return resp, up + " (udp,tc)", d, nil
	}

	if err != nil {
		if len(f.upstreams) > 1 {
			up2, _ := f.pickUpstream()
			start2 := time.Now()
			resp2, _, err2 := f.udpClient.ExchangeContext(ctx, req, up2)
			d = time.Since(start2)
			if err2 == nil && resp2 != nil {
				return resp2, up2 + " (udp,retry)", d, nil
			}
		}
		return nil, up, d, err
	}

	return resp, up + " (udp)", d, nil
}

func (f *DnsProxy) ServeDns(w dns.ResponseWriter, r *dns.Msg) {
	cacheHit := false

	now := time.Now()
	remote := w.RemoteAddr().String()
	network := w.RemoteAddr().Network()

	clientIp, clientPort := splitHostPort(remote)

	fail := func(rcode int) {
		m := new(dns.Msg)
		m.SetReply(r)
		m.RecursionAvailable = true
		m.Rcode = rcode
		_ = w.WriteMsg(m)
		f.logLine(now, network, clientIp, clientPort, r, nil, "local", 0, "fail", fmt.Sprintf("rcode=%d", rcode), cacheHit)
	}

	if r == nil || len(r.Question) == 0 {
		fail(dns.RcodeFormatError)
		return
	}

	var key string
	dnssecOk := doBit(r)

	if isCacheableQuery(r) {
		key = cacheKey(r.Question[0], dnssecOk)
		if msg, ok := f.cache.Get(key, now); ok {
			cacheHit = true
			msg.Id = r.Id
			msg.RecursionAvailable = true
			_ = w.WriteMsg(msg)
			f.logLine(now, network, clientIp, clientPort, r, msg, "cache", 0, "ok", "hit=true", cacheHit)
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), f.timeout)
	defer cancel()

	resp, upstreamUsed, latency, err := f.exchange(ctx, r)
	if err != nil || resp == nil {
		fail(dns.RcodeServerFailure)
		f.logLine(now, network, clientIp, clientPort, r, nil, upstreamUsed, latency, "fail", errString(err), cacheHit)
		return
	}

	resp.Id = r.Id
	resp.RecursionAvailable = true

	if err := w.WriteMsg(resp); err != nil {
		f.logLine(now, network, clientIp, clientPort, r, resp, upstreamUsed, latency, "fail", "write_failed="+err.Error(), cacheHit)
		return
	}

	if !cacheHit && key != "" {
		if ttl, ok := minTTL(resp); ok {
			f.cache.Put(key, resp, ttl, now)
		}
	}

	f.logLine(now, network, clientIp, clientPort, r, resp, upstreamUsed, latency, "ok", "hit=false", cacheHit)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func splitHostPort(addr string) (string, string) {
	h, p, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, ""
	}
	return h, p
}

func (f *DnsProxy) logLine(ts time.Time, netw, clientIP, clientPort string, req, resp *dns.Msg, upstream string, latency time.Duration, result, note string, cacheHit bool) {
	port := 0
	if clientPort != "" {
		if p, err := strconv.Atoi(clientPort); err == nil {
			port = p
		}
	}

	q := DnsQuestion{Name: "", Type: "", Class: ""}
	if req != nil && len(req.Question) > 0 {
		qq := req.Question[0]
		q.Name = dns.Fqdn(strings.ToLower(qq.Name))
		q.Type = dns.TypeToString[qq.Qtype]
		q.Class = dns.ClassToString[qq.Qclass]
	}

	var r *DnsResponse
	if resp != nil {
		r = &DnsResponse{
			Rcode:      dns.RcodeToString[resp.Rcode],
			Answers:    len(resp.Answer),
			Authority:  len(resp.Ns),
			Additional: len(resp.Extra),
			Truncated:  resp.Truncated,
		}
	}

	// get container id and name
	var (
		containerId   string
		containerName string
		veth          string
		spiffeId      string
	)
	containerId, veth, err := f.ipamHandler.GetInfoByIp(clientIP)
	if err != nil {
		containerId = "unresolved"
		containerName = "unresolved"
		spiffeId = "unresolved"
		veth = "unresolved"
	} else {
		containerName, err = f.csmHandler.GetContainerNameById(containerId)
		if err != nil {
			containerName = "unresolved"
		}
		spiffeId, err = f.csmHandler.GetSpiffeById(containerId)
		if err != nil {
			spiffeId = "unresolved"
		}
	}

	ev := DnsEvent{
		Ts:        ts.Format(time.RFC3339Nano),
		EventType: "log.traffic",
		Network: Network{
			Transport: netw,
		},
		Client: Client{
			Ip:            clientIP,
			Port:          port,
			ContainerId:   containerId,
			ContainerName: containerName,
			SpiffeId:      spiffeId,
			Veth:          veth,
		},
		Dns: DnsBlock{
			Id: func() uint16 {
				if req != nil {
					return req.Id
				}
				return 0
			}(),
			Rd: func() bool {
				if req != nil {
					return req.RecursionDesired
				}
				return false
			}(),
			Question: q,
			Response: r,
		},
		LatencyMs: latency.Milliseconds(),
		Cache: &CacheInfo{
			Hit: cacheHit,
		},
		Result: result,
		Note:   note,
	}

	if upstream != "" {
		splitUpstream := func(s string) (addr string, proto string) {
			s = strings.TrimSpace(s)
			if i := strings.LastIndex(s, " "); i != -1 {
				addr = s[:i]
				p := strings.TrimSpace(s[i+1:])
				proto = strings.Trim(p, "()")
				return
			}
			return s, ""
		}

		server, proto := splitUpstream(upstream)

		ev.Upstream = &UpstreamInfo{
			Server:    server,
			Transport: proto,
		}
	}

	_ = f.logger.Write(ev)
}

func escapeForJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
