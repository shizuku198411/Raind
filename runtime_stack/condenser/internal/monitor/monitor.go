package monitor

import (
	"condenser/internal/store/csm"
	"condenser/internal/store/psm"
	"condenser/internal/utils"
	"context"
	"log"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

func NewContainerMonitor() *ContainerMonitor {
	return &ContainerMonitor{
		csmHandler: csm.NewCsmManager(csm.NewCsmStore(utils.CsmStorePath)),
		psmHandler: psm.NewPsmManager(psm.NewPsmStore(utils.PsmStorePath)),
	}
}

type ContainerMonitor struct {
	csmHandler csm.CsmHandler
	psmHandler psm.PsmHandler
}

func (m *ContainerMonitor) Start() error {
	resolver := NewResolver(m.csmHandler)
	metricsWriter, err := NewMetricsWriter(utils.MetricsLogPath)
	if err != nil {
		return err
	}
	defer metricsWriter.Close()
	prevCPU := map[string]CgroupCPUUsageSample{}

	// watch CSM file update
	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if err := resolver.Watch(ctx); err != nil {
			log.Printf("watch stopped: %v", err)
		}
	}()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	metricTick := 0

	for range ticker.C {
		metricTick++
		for _, container := range resolver.ResolveMap {
			// status check
			// monitoring target: created, running
			if container.Status != "running" && container.Status != "created" {
				continue
			}
			// send keep alive
			procExist, _ := m.pidAlive(container.Pid)
			// if process is not exist, change state to stopped
			if !procExist {
				log.Printf("[*] Container: %s down detected.", container.ContainerId)
				if err := m.csmHandler.UpdateContainer(
					container.ContainerId,
					"stopped",
					0,
				); err != nil {
					continue
				}
				if container.PodId != "" && !strings.HasPrefix(container.ContainerName, utils.PodInfraContainerNamePrefix) {
					_ = m.psmHandler.UpdatePod(container.PodId, "degraded")
				}
				if err := m.csmHandler.UpdateExitStatus(
					container.ContainerId,
					-1,
					"Error",
					"process down detected.",
				); err != nil {
					continue
				}
				continue
			}

			if container.Status != "running" {
				continue
			}

			if metricTick%10 == 0 {
				record, sample, err := buildMetricsRecord(container, prevCPU[container.ContainerId])
				if err != nil {
					log.Printf("metrics build failed: container=%s err=%v", container.ContainerId, err)
					continue
				}
				prevCPU[container.ContainerId] = sample
				if err := metricsWriter.WriteJSONL(record); err != nil {
					log.Printf("metrics write failed: container=%s err=%v", container.ContainerId, err)
				}
			}
		}
	}
	return nil
}

func buildMetricsRecord(container ContainerMeta, prev CgroupCPUUsageSample) (MetricsRecord, CgroupCPUUsageSample, error) {
	cgroupPath := CgroupV2Path(container.ContainerId)

	cpuStat, err := ReadCgroupCPUStat(cgroupPath)
	if err != nil {
		return MetricsRecord{}, CgroupCPUUsageSample{}, err
	}
	cpuQuota, err := ReadCgroupCPUQuota(cgroupPath)
	if err != nil {
		return MetricsRecord{}, CgroupCPUUsageSample{}, err
	}
	curSample := CgroupCPUUsageSample{
		UsageUsec: cpuStat.UsageUsec,
		Timestamp: time.Now(),
	}
	cpuPercent := 0.0
	if !prev.Timestamp.IsZero() {
		cpuPercent = CgroupCPUPercent(prev, curSample, cpuQuota)
	}
	memStat, err := ReadCgroupMemoryStat(cgroupPath)
	if err != nil {
		return MetricsRecord{}, CgroupCPUUsageSample{}, err
	}
	ioStat, err := ReadCgroupIOStat(cgroupPath)
	if err != nil {
		return MetricsRecord{}, CgroupCPUUsageSample{}, err
	}
	memEvents, err := ReadCgroupMemoryEvents(cgroupPath)
	if err != nil {
		return MetricsRecord{}, CgroupCPUUsageSample{}, err
	}
	memPercent, memLimited := CgroupMemoryPercent(memStat)

	return MetricsRecord{
		GeneratedTS: time.Now().Format(time.RFC3339Nano),

		ContainerID:   container.ContainerId,
		ContainerName: container.ContainerName,
		SpiffeID:      container.SpiffeId,
		Pid:           container.Pid,
		Status:        container.Status,
		CgroupPath:    cgroupPath,

		CPUUsageUsec:     cpuStat.UsageUsec,
		CPUUserUsec:      cpuStat.UserUsec,
		CPUSystemUsec:    cpuStat.SystemUsec,
		CPUNrPeriods:     cpuStat.NrPeriods,
		CPUNrThrottled:   cpuStat.NrThrottled,
		CPUThrottledUsec: cpuStat.ThrottledUsec,
		CPUQuotaUsec:     cpuQuota.QuotaUsec,
		CPUPeriodUsec:    cpuQuota.PeriodUsec,
		CPUUnlimited:     cpuQuota.Unlimited,
		CPUPercent:       cpuPercent,

		MemoryCurrentBytes: memStat.CurrentBytes,
		MemoryMaxBytes:     memStat.MaxBytes,
		MemoryLimited:      memLimited,
		MemoryPercent:      memPercent,

		IOReadBytes:  ioStat.RBytes,
		IOWriteBytes: ioStat.WBytes,
		IOReadOps:    ioStat.RIOs,
		IOWriteOps:   ioStat.WIOs,

		MemoryOOM:     memEvents.OOM,
		MemoryOOMKill: memEvents.OOMKill,
	}, curSample, nil
}

func (m *ContainerMonitor) pidAlive(pid int) (bool, error) {
	if pid <= 0 {
		// process not exist
		return false, nil
	}

	// send 0 signal to process
	err := syscall.Kill(pid, 0)
	switch err {
	case nil:
		// process exist
		return true, nil
	case syscall.ESRCH:
		// no such process
		return false, nil
	case syscall.EPERM:
		// operation not permitted, but process exist
		return true, nil
	}
	// other signal: process not exist
	return false, nil
}

func NewResolver(csmHandler csm.CsmHandler) *Resolver {
	resolver := &Resolver{
		ResolveMap: map[string]ContainerMeta{},
		csmHandler: csmHandler,
	}
	containerList, _ := csmHandler.GetContainerList()
	for _, c := range containerList {
		if _, ok := resolver.ResolveMap[c.ContainerId]; !ok {
			resolver.ResolveMap[c.ContainerId] = ContainerMeta{
				ContainerId:   c.ContainerId,
				ContainerName: c.ContainerName,
				PodId:         c.PodId,
				SpiffeId:      c.SpiffeId,
				Status:        c.State,
				Pid:           c.Pid,
			}
		}
	}
	return resolver
}

type Resolver struct {
	ResolveMap map[string]ContainerMeta
	csmHandler csm.CsmHandler
}

func (r *Resolver) Refresh() {
	r.ResolveMap = map[string]ContainerMeta{}
	containerList, _ := r.csmHandler.GetContainerList()
	for _, c := range containerList {
		if _, ok := r.ResolveMap[c.ContainerId]; !ok {
			r.ResolveMap[c.ContainerId] = ContainerMeta{
				ContainerId:   c.ContainerId,
				ContainerName: c.ContainerName,
				PodId:         c.PodId,
				SpiffeId:      c.SpiffeId,
				Status:        c.State,
				Pid:           c.Pid,
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
