package deployment

import "time"

type DeploymentInfoModel struct {
	DeploymentId string    `json:"deploymentId"`
	Name         string    `json:"name"`
	Namespace    string    `json:"namespace"`
	Replicas     int       `json:"replicas"`
	TemplateId   string    `json:"templateId,omitempty"`
	ReplicaSetId string    `json:"replicaSetId,omitempty"`
	Desired      int       `json:"desired"`
	Current      int       `json:"current"`
	Ready        int       `json:"ready"`
	CreatedAt    time.Time `json:"createdAt"`
}

type ListResponseModel struct {
	Status  string                `json:"status"`
	Message string                `json:"message"`
	Data    []DeploymentInfoModel `json:"data,omitempty"`
}
