package pod

import "raind/internal/condenser/store/psm"

type CreatePodRequest struct {
	Name        string                      `json:"name"`
	Namespace   string                      `json:"namespace"`
	UID         string                      `json:"uid"`
	NetworkNS   string                      `json:"networkNS"`
	IPCNS       string                      `json:"ipcNS"`
	UTSNS       string                      `json:"utsNS"`
	UserNS      string                      `json:"userNS"`
	HostUsers   *bool                       `json:"hostUsers,omitempty"`
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
	CapAdd  []string `json:"capAdd,omitempty"`
	CapDrop []string `json:"capDrop,omitempty"`
	Network string   `json:"network"`
	Tty     bool     `json:"tty"`
}

type CreatePodResponse struct {
	PodId string `json:"podId"`
}

type ApplyPodResponse struct {
	Pods        []ApplyPodResult        `json:"pods"`
	ReplicaSets []ApplyReplicaSetResult `json:"replicasets"`
	Deployments []ApplyDeploymentResult `json:"deployments"`
	Services    []ApplyServiceResult    `json:"services"`
	Ingresses   []ApplyIngressResult    `json:"ingresses"`
	Namespaces  []ApplyNamespaceResult  `json:"namespaces"`
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

type ApplyIngressResult struct {
	IngressId string   `json:"ingressId"`
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	TLSHosts  []string `json:"tlsHosts,omitempty"`
}

type ApplyNamespaceResult struct {
	Name    string `json:"name"`
	Network string `json:"network"`
}

type ApplyReplicaSetResult struct {
	ReplicaSetId string `json:"replicaSetId"`
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
}

type ApplyDeploymentResult struct {
	DeploymentId string `json:"deploymentId"`
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Replicas     int    `json:"replicas"`
}

type DeleteResourcesResponse struct {
	Pods        []DeletePodResult        `json:"pods"`
	ReplicaSets []DeleteReplicaSetResult `json:"replicasets"`
	Deployments []DeleteDeploymentResult `json:"deployments"`
	Services    []DeleteServiceResult    `json:"services"`
	Ingresses   []DeleteIngressResult    `json:"ingresses"`
	Namespaces  []DeleteNamespaceResult  `json:"namespaces"`
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

type DeleteIngressResult struct {
	IngressId string `json:"ingressId"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type DeleteDeploymentResult struct {
	DeploymentId string `json:"deploymentId"`
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
}

type DeleteNamespaceResult struct {
	Name string `json:"name"`
}

type ScaleDeploymentRequest struct {
	Replicas int `json:"replicas"`
}

type ScaleDeploymentResponse struct {
	DeploymentId string `json:"deploymentId"`
	Replicas     int    `json:"replicas"`
}

type DeploymentSummary struct {
	DeploymentId string            `json:"deploymentId"`
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Replicas     int               `json:"replicas"`
	Desired      int               `json:"desired"`
	Current      int               `json:"current"`
	Ready        int               `json:"ready"`
	ReplicaSetId string            `json:"replicaSetId,omitempty"`
	TemplateId   string            `json:"templateId,omitempty"`
	Selector     map[string]string `json:"selector,omitempty"`
	CreatedAt    string            `json:"createdAt"`
	UpdatedAt    string            `json:"updatedAt"`
}

type DeploymentDetail struct {
	DeploymentId string              `json:"deploymentId"`
	Name         string              `json:"name"`
	Namespace    string              `json:"namespace"`
	Replicas     int                 `json:"replicas"`
	Desired      int                 `json:"desired"`
	Current      int                 `json:"current"`
	Ready        int                 `json:"ready"`
	ReplicaSetId string              `json:"replicaSetId,omitempty"`
	Selector     map[string]string   `json:"selector,omitempty"`
	Template     psm.PodTemplateSpec `json:"template"`
	CreatedAt    string              `json:"createdAt"`
	UpdatedAt    string              `json:"updatedAt"`
}

type ScaleReplicaSetRequest struct {
	Replicas int `json:"replicas"`
}

type ScaleReplicaSetResponse struct {
	ReplicaSetId string `json:"replicaSetId"`
	Replicas     int    `json:"replicas"`
}

type ReplicaSetSummary struct {
	ReplicaSetId       string            `json:"replicaSetId"`
	Name               string            `json:"name"`
	Namespace          string            `json:"namespace"`
	Replicas           int               `json:"replicas"`
	Desired            int               `json:"desired"`
	Current            int               `json:"current"`
	Ready              int               `json:"ready"`
	TemplateId         string            `json:"templateId"`
	Selector           map[string]string `json:"selector,omitempty"`
	ReconcileAttempt   int               `json:"reconcileAttempt,omitempty"`
	LastReconcileError string            `json:"lastReconcileError,omitempty"`
	NextReconcileAt    string            `json:"nextReconcileAt,omitempty"`
	CreatedAt          string            `json:"createdAt"`
}

type ReplicaSetDetail struct {
	ReplicaSetId       string              `json:"replicaSetId"`
	Name               string              `json:"name"`
	Namespace          string              `json:"namespace"`
	Replicas           int                 `json:"replicas"`
	Desired            int                 `json:"desired"`
	Current            int                 `json:"current"`
	Ready              int                 `json:"ready"`
	Selector           map[string]string   `json:"selector,omitempty"`
	Template           psm.PodTemplateSpec `json:"template"`
	ReconcileAttempt   int                 `json:"reconcileAttempt,omitempty"`
	LastReconcileError string              `json:"lastReconcileError,omitempty"`
	NextReconcileAt    string              `json:"nextReconcileAt,omitempty"`
	CreatedAt          string              `json:"createdAt"`
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
