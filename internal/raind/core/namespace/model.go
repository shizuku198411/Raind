package namespace

import "time"

type NamespaceInfoModel struct {
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

type CreateModel struct {
	Name    string
	Network string
}

type CreateRequestModel struct {
	Name    string `json:"name"`
	Network string `json:"network,omitempty"`
}

type CreateResponseModel struct {
	Status  string             `json:"status"`
	Message string             `json:"message"`
	Data    NamespaceInfoModel `json:"data"`
}

type DetailResponseModel struct {
	Status  string             `json:"status"`
	Message string             `json:"message"`
	Data    NamespaceInfoModel `json:"data"`
}

type ListResponseModel struct {
	Status  string               `json:"status"`
	Message string               `json:"message"`
	Data    []NamespaceInfoModel `json:"data"`
}

type RemoveResponseModel struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
