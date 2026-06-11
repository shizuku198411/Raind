package replicaset

import "time"

type ReplicaSetContainerModel struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	Tty   bool   `json:"tty,omitempty"`
}

type ReplicaSetTemplateModel struct {
	Name       string                     `json:"name"`
	Namespace  string                     `json:"namespace"`
	NetworkNS  string                     `json:"networkNS"`
	IpcNS      string                     `json:"ipcNS"`
	UtsNS      string                     `json:"utsNS"`
	UserNS     string                     `json:"userNS"`
	Labels     map[string]string          `json:"labels,omitempty"`
	Containers []ReplicaSetContainerModel `json:"containers,omitempty"`
}

type ReplicaSetDetailModel struct {
	ReplicaSetId string                  `json:"replicaSetId"`
	Name         string                  `json:"name"`
	Namespace    string                  `json:"namespace"`
	Replicas     int                     `json:"replicas"`
	Template     ReplicaSetTemplateModel `json:"template"`
	CreatedAt    time.Time               `json:"createdAt"`
}

type DetailResponseModel struct {
	Status  string                `json:"status"`
	Message string                `json:"message"`
	Data    ReplicaSetDetailModel `json:"data,omitempty"`
}
