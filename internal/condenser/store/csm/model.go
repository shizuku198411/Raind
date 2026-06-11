package csm

import "time"

type ContainerInfo struct {
	ContainerId   string            `json:"containerId"`
	ContainerName string            `json:"name"`
	PodId         string            `json:"podId,omitempty"`
	SpiffeId      string            `json:"spiffeId"`
	State         string            `json:"state"`
	Pid           int               `json:"pid"`
	ExitCode      int               `json:"exit_code"` // CRI required
	Reason        string            `json:"reason"`    // CRI required
	Message       string            `json:"message"`   // CRI required
	LogPath       string            `json:"log_path"`  // CRI required
	Tty           bool              `json:"tty"`
	Repository    string            `json:"imageRepository"`
	Reference     string            `json:"imageReference"`
	Command       []string          `json:"command"`
	BottleId      string            `json:"bottleId,omitempty"`
	CreatingAt    time.Time         `json:"creatingAt"`
	CreatedAt     time.Time         `json:"createdAt"`
	StartedAt     time.Time         `json:"statedAt"`
	StoppedAt     time.Time         `json:"stoppedAt"`
	FinishedAt    time.Time         `json:"finished_at"` // CRI required
	Labels        map[string]string `json:"labels"`      // CRI required
	Annotaions    map[string]string `json:"annotations"` // CRI required
	Attemp        uint32            `json:"attempt"`     // CRI required
}

type ContainerState struct {
	Version    string                   `json:"version"`
	Containers map[string]ContainerInfo `json:"containers"`
}
