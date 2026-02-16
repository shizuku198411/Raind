package replicaset

import "time"

type ReplicaSetInfoModel struct {
	ReplicaSetId string    `json:"replicaSetId"`
	Name         string    `json:"name"`
	Namespace    string    `json:"namespace"`
	Replicas     int       `json:"replicas"`
	TemplateId   string    `json:"templateId,omitempty"`
	Desired      int       `json:"desired"`
	Current      int       `json:"current"`
	Ready        int       `json:"ready"`
	CreatedAt    time.Time `json:"createdAt"`
}

type ListResponseModel struct {
	Status  string                `json:"status"`
	Message string                `json:"message"`
	Data    []ReplicaSetInfoModel `json:"data,omitempty"`
}
