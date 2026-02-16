package monitor

type ContainerMeta struct {
	ContainerId   string
	ContainerName string
	PodId         string
	SpiffeId      string
	Status        string
	Pid           int
}

type MetricsRecord struct {
	GeneratedTS string `json:"generated_ts"`

	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name"`
	SpiffeID      string `json:"spiffe_id"`
	Pid           int    `json:"pid"`
	Status        string `json:"status"`
	CgroupPath    string `json:"cgroup_path"`

	CPUUsageUsec     uint64  `json:"cpu_usage_usec"`
	CPUUserUsec      uint64  `json:"cpu_user_usec"`
	CPUSystemUsec    uint64  `json:"cpu_system_usec"`
	CPUNrPeriods     uint64  `json:"cpu_nr_periods"`
	CPUNrThrottled   uint64  `json:"cpu_nr_throttled"`
	CPUThrottledUsec uint64  `json:"cpu_throttled_usec"`
	CPUQuotaUsec     uint64  `json:"cpu_quota_usec"`
	CPUPeriodUsec    uint64  `json:"cpu_period_usec"`
	CPUUnlimited     bool    `json:"cpu_unlimited"`
	CPUPercent       float64 `json:"cpu_percent"`

	MemoryCurrentBytes uint64  `json:"memory_current_bytes"`
	MemoryMaxBytes     *uint64 `json:"memory_max_bytes"`
	MemoryLimited      bool    `json:"memory_limited"`
	MemoryPercent      float64 `json:"memory_percent"`

	IOReadBytes  uint64 `json:"io_read_bytes"`
	IOWriteBytes uint64 `json:"io_write_bytes"`
	IOReadOps    uint64 `json:"io_read_ops"`
	IOWriteOps   uint64 `json:"io_write_ops"`

	MemoryOOM     uint64 `json:"memory_oom"`
	MemoryOOMKill uint64 `json:"memory_oom_kill"`
}
