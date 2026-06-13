package namespace

import "time"

type ServiceCreateModel struct {
	Name        string
	Network     string
	Labels      map[string]string
	Annotations map[string]string
}

type ServiceRemoveModel struct {
	Name string
}

type NamespaceInfo struct {
	Name        string            `json:"name"`
	Network     string            `json:"network"`
	NetworkAuto bool              `json:"networkAuto"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	Resources   ResourceCounts    `json:"resources"`
}

type ResourceCounts struct {
	Pods        int `json:"pods"`
	Services    int `json:"services"`
	ReplicaSets int `json:"replicasets"`
	Deployments int `json:"deployments"`
	Allocations int `json:"allocations"`
}
