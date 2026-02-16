package container

import "time"

type ServiceCreateModel struct {
	Image      string
	Os         string
	Arch       string
	Command    []string
	Port       []string
	Mount      []string
	Env        []string
	Network    string
	Tty        bool
	Name       string
	BottleId   string
	PodId      string
	IsPodInfra bool
}

type ServiceStartModel struct {
	ContainerId string
	Tty         bool
	OpBottle    bool
}

type ServiceDeleteModel struct {
	ContainerId string
	OpBottle    bool
}

type ServiceStopModel struct {
	ContainerId string
	OpBottle    bool
}

type ServiceExecModel struct {
	ContainerId string
	Tty         bool
	Entrypoint  []string
}

type ForwardInfo struct {
	HostPort      int    `json:"source"`
	ContainerPort int    `json:"destination"`
	Protocol      string `json:"protocol"`
}

type ContainerState struct {
	ContainerId string   `json:"containerId"`
	Name        string   `json:"name"`
	PodId       string   `json:"podId,omitempty"`
	State       string   `json:"state"`
	Pid         int      `json:"pid"`
	Repository  string   `json:"imageRepository"`
	Reference   string   `json:"imageReference"`
	Command     []string `json:"command"`

	Address  string        `json:"address"`
	Forwards []ForwardInfo `json:"forwards"`

	CreatingAt time.Time `json:"creatingAt"`
	CreatedAt  time.Time `json:"createdAt"`
	StartedAt  time.Time `json:"statedAt"`
	StoppedAt  time.Time `json:"stoppedAt"`
}

type ContainerStats struct {
	GeneratedTS string `json:"generated_ts"`

	ContainerID   string            `json:"container_id"`
	ContainerName string            `json:"container_name"`
	PodId         string            `json:"pod_id"`
	SpiffeID      string            `json:"spiffe_id"`
	Pid           int               `json:"pid"`
	Status        string            `json:"status"`
	CgroupPath    string            `json:"cgroup_path"`
	ExitCode      int               `json:"exit_code"`
	Reason        string            `json:"reason"`
	Message       string            `json:"message"`
	LogPath       string            `json:"log_path"`
	Repository    string            `json:"image_repository"`
	Reference     string            `json:"image_reference"`
	Command       []string          `json:"command"`
	BottleId      string            `json:"bottle_id,omitempty"`
	CreatingAt    time.Time         `json:"creating_at"`
	CreatedAt     time.Time         `json:"created_at"`
	StartedAt     time.Time         `json:"started_at"`
	StoppedAt     time.Time         `json:"stopped_at"`
	FinishedAt    time.Time         `json:"finished_at"`
	Labels        map[string]string `json:"labels"`
	Annotations   map[string]string `json:"annotations"`
	Attempt       uint32            `json:"attempt"`
	Tty           bool              `json:"tty"`

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
