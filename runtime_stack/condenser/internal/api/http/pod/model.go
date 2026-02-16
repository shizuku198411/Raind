package pod

import "condenser/internal/store/psm"

type CreatePodRequest struct {
	Name        string                      `json:"name"`
	Namespace   string                      `json:"namespace"`
	UID         string                      `json:"uid"`
	NetworkNS   string                      `json:"networkNS"`
	IPCNS       string                      `json:"ipcNS"`
	UTSNS       string                      `json:"utsNS"`
	UserNS      string                      `json:"userNS"`
	Labels      map[string]string           `json:"labels"`
	Annotations map[string]string           `json:"annotations"`
	Containers  []CreatePodContainerRequest `json:"containers"`
}

type CreatePodContainerRequest struct {
	Name    string   `json:"name"`
	Image   string   `json:"image"`
	Command []string `json:"command"`
	Port    []string `json:"port"`
	Mount   []string `json:"mount"`
	Env     []string `json:"env"`
	Network string   `json:"network"`
	Tty     bool     `json:"tty"`
}

type CreatePodResponse struct {
	PodId string `json:"podId"`
}

type ApplyPodResponse struct {
	Pods     []ApplyPodResult     `json:"pods"`
	Services []ApplyServiceResult `json:"services"`
}

type ApplyPodResult struct {
	PodId        string   `json:"podId"`
	ReplicaSetId string   `json:"replicaSetId,omitempty"`
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	ContainerIds []string `json:"containerIds"`
}

type ApplyServiceResult struct {
	ServiceId string `json:"serviceId"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type DeleteResourcesResponse struct {
	Pods        []DeletePodResult        `json:"pods"`
	ReplicaSets []DeleteReplicaSetResult `json:"replicasets"`
	Services    []DeleteServiceResult    `json:"services"`
}

type DeletePodResult struct {
	PodId     string `json:"podId"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type DeleteReplicaSetResult struct {
	ReplicaSetId string `json:"replicaSetId"`
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
}

type DeleteServiceResult struct {
	ServiceId string `json:"serviceId"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type ScaleReplicaSetRequest struct {
	Replicas int `json:"replicas"`
}

type ScaleReplicaSetResponse struct {
	ReplicaSetId string `json:"replicaSetId"`
	Replicas     int    `json:"replicas"`
}

type ReplicaSetSummary struct {
	ReplicaSetId string            `json:"replicaSetId"`
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Replicas     int               `json:"replicas"`
	Desired      int               `json:"desired"`
	Current      int               `json:"current"`
	Ready        int               `json:"ready"`
	TemplateId   string            `json:"templateId"`
	Selector     map[string]string `json:"selector,omitempty"`
	CreatedAt    string            `json:"createdAt"`
}

type ReplicaSetDetail struct {
	ReplicaSetId string              `json:"replicaSetId"`
	Name         string              `json:"name"`
	Namespace    string              `json:"namespace"`
	Replicas     int                 `json:"replicas"`
	Desired      int                 `json:"desired"`
	Current      int                 `json:"current"`
	Ready        int                 `json:"ready"`
	Selector     map[string]string   `json:"selector,omitempty"`
	Template     psm.PodTemplateSpec `json:"template"`
	CreatedAt    string              `json:"createdAt"`
}

type StartPodResponse struct {
	PodId string `json:"podId"`
}

type StopPodResponse struct {
	PodId string `json:"podId"`
}

type RemovePodResponse struct {
	PodId string `json:"podId"`
}
