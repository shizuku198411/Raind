package monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"condenser/internal/utils"
)

// CgroupV2Path returns the container cgroup v2 path under the Raind root.
func CgroupV2Path(containerID string) string {
	return filepath.Join(utils.CgroupRuntimeDir, containerID)
}

// CgroupCPUUsageSample is a snapshot of cpu.stat usage.
type CgroupCPUUsageSample struct {
	UsageUsec uint64
	Timestamp time.Time
}

// CgroupCPUStat is parsed from cpu.stat.
type CgroupCPUStat struct {
	UsageUsec     uint64
	UserUsec      uint64
	SystemUsec    uint64
	NrPeriods     uint64
	NrThrottled   uint64
	ThrottledUsec uint64
}

// CgroupCPUQuota is parsed from cpu.max.
type CgroupCPUQuota struct {
	QuotaUsec  uint64
	PeriodUsec uint64
	Unlimited  bool
}

// CgroupMemoryStat is parsed from memory.current/memory.max.
type CgroupMemoryStat struct {
	CurrentBytes uint64
	MaxBytes     *uint64 // nil means "max" (unlimited)
}

// CgroupIOStat is parsed from io.stat.
type CgroupIOStat struct {
	RBytes uint64
	WBytes uint64
	RIOs   uint64
	WIOs   uint64
}

// CgroupMemoryEvents is parsed from memory.events.
type CgroupMemoryEvents struct {
	OOM     uint64
	OOMKill uint64
}

// ReadCgroupCPUStat reads cpu.stat from a cgroup v2 path.
func ReadCgroupCPUStat(cgroupPath string) (CgroupCPUStat, error) {
	b, err := os.ReadFile(filepath.Join(cgroupPath, "cpu.stat"))
	if err != nil {
		return CgroupCPUStat{}, err
	}
	values := parseKeyValueLines(string(b))
	return CgroupCPUStat{
		UsageUsec:     values["usage_usec"],
		UserUsec:      values["user_usec"],
		SystemUsec:    values["system_usec"],
		NrPeriods:     values["nr_periods"],
		NrThrottled:   values["nr_throttled"],
		ThrottledUsec: values["throttled_usec"],
	}, nil
}

// ReadCgroupCPUQuota reads cpu.max from a cgroup v2 path.
func ReadCgroupCPUQuota(cgroupPath string) (CgroupCPUQuota, error) {
	b, err := os.ReadFile(filepath.Join(cgroupPath, "cpu.max"))
	if err != nil {
		return CgroupCPUQuota{}, err
	}
	fields := strings.Fields(strings.TrimSpace(string(b)))
	if len(fields) < 2 {
		return CgroupCPUQuota{}, fmt.Errorf("invalid cpu.max format")
	}
	if fields[0] == "max" {
		period, _ := strconv.ParseUint(fields[1], 10, 64)
		return CgroupCPUQuota{QuotaUsec: 0, PeriodUsec: period, Unlimited: true}, nil
	}
	quota, _ := strconv.ParseUint(fields[0], 10, 64)
	period, _ := strconv.ParseUint(fields[1], 10, 64)
	return CgroupCPUQuota{QuotaUsec: quota, PeriodUsec: period, Unlimited: false}, nil
}

// SampleCgroupCPUUsage reads usage_usec for CPU usage rate calculations.
func SampleCgroupCPUUsage(cgroupPath string) (CgroupCPUUsageSample, error) {
	stat, err := ReadCgroupCPUStat(cgroupPath)
	if err != nil {
		return CgroupCPUUsageSample{}, err
	}
	return CgroupCPUUsageSample{
		UsageUsec: stat.UsageUsec,
		Timestamp: time.Now(),
	}, nil
}

// CgroupCPUPercent returns CPU usage percent based on cpu.stat and cpu.max.
// If quota is unlimited, it normalizes by host CPU cores.
func CgroupCPUPercent(prev, cur CgroupCPUUsageSample, quota CgroupCPUQuota) float64 {
	if cur.Timestamp.Before(prev.Timestamp) || cur.UsageUsec < prev.UsageUsec {
		return 0
	}
	intervalUsec := cur.Timestamp.Sub(prev.Timestamp).Microseconds()
	if intervalUsec <= 0 {
		return 0
	}
	usedCores := float64(cur.UsageUsec-prev.UsageUsec) / float64(intervalUsec)
	maxCores := float64(runtime.NumCPU())
	if !quota.Unlimited && quota.PeriodUsec > 0 && quota.QuotaUsec > 0 {
		maxCores = float64(quota.QuotaUsec) / float64(quota.PeriodUsec)
	}
	if maxCores <= 0 {
		return 0
	}
	return (usedCores / maxCores) * 100.0
}

// ReadCgroupMemoryStat reads memory.current and memory.max from a cgroup v2 path.
func ReadCgroupMemoryStat(cgroupPath string) (CgroupMemoryStat, error) {
	curB, err := os.ReadFile(filepath.Join(cgroupPath, "memory.current"))
	if err != nil {
		return CgroupMemoryStat{}, err
	}
	cur, _ := strconv.ParseUint(strings.TrimSpace(string(curB)), 10, 64)

	maxB, err := os.ReadFile(filepath.Join(cgroupPath, "memory.max"))
	if err != nil {
		return CgroupMemoryStat{}, err
	}
	maxStr := strings.TrimSpace(string(maxB))
	if maxStr == "max" {
		return CgroupMemoryStat{CurrentBytes: cur, MaxBytes: nil}, nil
	}
	max, _ := strconv.ParseUint(maxStr, 10, 64)
	return CgroupMemoryStat{CurrentBytes: cur, MaxBytes: &max}, nil
}

// CgroupMemoryPercent returns memory usage percent and whether a max is set.
func CgroupMemoryPercent(stat CgroupMemoryStat) (float64, bool) {
	if stat.MaxBytes == nil || *stat.MaxBytes == 0 {
		return 0, false
	}
	return (float64(stat.CurrentBytes) / float64(*stat.MaxBytes)) * 100.0, true
}

// ReadCgroupIOStat reads io.stat and aggregates rbytes/wbytes/rios/wios.
func ReadCgroupIOStat(cgroupPath string) (CgroupIOStat, error) {
	b, err := os.ReadFile(filepath.Join(cgroupPath, "io.stat"))
	if err != nil {
		if os.IsNotExist(err) {
			return CgroupIOStat{}, nil
		}
		return CgroupIOStat{}, err
	}
	var out CgroupIOStat
	for _, line := range strings.Split(string(b), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		for _, f := range fields[1:] {
			kv := strings.SplitN(f, "=", 2)
			if len(kv) != 2 {
				continue
			}
			v, err := strconv.ParseUint(kv[1], 10, 64)
			if err != nil {
				continue
			}
			switch kv[0] {
			case "rbytes":
				out.RBytes += v
			case "wbytes":
				out.WBytes += v
			case "rios":
				out.RIOs += v
			case "wios":
				out.WIOs += v
			}
		}
	}
	return out, nil
}

// ReadCgroupMemoryEvents reads memory.events.
func ReadCgroupMemoryEvents(cgroupPath string) (CgroupMemoryEvents, error) {
	b, err := os.ReadFile(filepath.Join(cgroupPath, "memory.events"))
	if err != nil {
		return CgroupMemoryEvents{}, err
	}
	values := parseKeyValueLines(string(b))
	return CgroupMemoryEvents{
		OOM:     values["oom"],
		OOMKill: values["oom_kill"],
	}, nil
}

func parseKeyValueLines(body string) map[string]uint64 {
	out := make(map[string]uint64)
	for _, line := range strings.Split(body, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		v, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		out[fields[0]] = v
	}
	return out
}
