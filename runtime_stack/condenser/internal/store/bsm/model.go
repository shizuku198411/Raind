package bsm

import "time"

type BottleState struct {
	Version string                `json:"version"`
	Bottles map[string]BottleInfo `json:"bottles"`
}

type BottleInfo struct {
	BottleId   string                 `json:"bottleId"`
	BottleName string                 `json:"bottleName"`
	Services   map[string]ServiceSpec `json:"services"`
	StartOrder []string               `json:"startOrder"`
	Containers map[string]string      `json:"containers"`
	Policies   []PolicyInfo           `json:"policies,omitempty"`
	Network    string                 `json:"network,omitempty"`
	NetworkAuto bool                  `json:"networkAuto,omitempty"`
	CreatedAt  time.Time              `json:"createdAt"`
}

type ServiceSpec struct {
	Image     string   `json:"image"`
	Command   []string `json:"command,omitempty"`
	Env       []string `json:"env,omitempty"`
	Ports     []string `json:"ports,omitempty"`
	Mount     []string `json:"mount,omitempty"`
	Network   string   `json:"network,omitempty"`
	Tty       bool     `json:"tty,omitempty"`
	DependsOn []string `json:"dependsOn,omitempty"`
}

type PolicyInfo struct {
	Id          string `json:"id"`
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Protocol    string `json:"protocol,omitempty"`
	DestPort    int    `json:"destPort,omitempty"`
	Comment     string `json:"comment,omitempty"`
}
