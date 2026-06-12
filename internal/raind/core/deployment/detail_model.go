package deployment

import "time"

type DeploymentContainerModel struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	Tty   bool   `json:"tty,omitempty"`
}

type DeploymentTemplateModel struct {
	Name       string                     `json:"name"`
	Namespace  string                     `json:"namespace"`
	NetworkNS  string                     `json:"networkNS"`
	IpcNS      string                     `json:"ipcNS"`
	UtsNS      string                     `json:"utsNS"`
	UserNS     string                     `json:"userNS"`
	Labels     map[string]string          `json:"labels,omitempty"`
	Containers []DeploymentContainerModel `json:"containers,omitempty"`
}

type DeploymentDetailModel struct {
	DeploymentId string                  `json:"deploymentId"`
	Name         string                  `json:"name"`
	Namespace    string                  `json:"namespace"`
	Replicas     int                     `json:"replicas"`
	Desired      int                     `json:"desired"`
	Current      int                     `json:"current"`
	Ready        int                     `json:"ready"`
	ReplicaSetId string                  `json:"replicaSetId,omitempty"`
	Template     DeploymentTemplateModel `json:"template"`
	CreatedAt    time.Time               `json:"createdAt"`
	UpdatedAt    time.Time               `json:"updatedAt"`
}

type DetailResponseModel struct {
	Status  string                `json:"status"`
	Message string                `json:"message"`
	Data    DeploymentDetailModel `json:"data,omitempty"`
}
