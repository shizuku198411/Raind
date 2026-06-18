package psm

import "time"

const (
	PodStateCreated  = "created"
	PodStateRunning  = "running"
	PodStateStopped  = "stopped"
	PodStateDegraded = "degraded"

	ContainerStateCreated = "created"
	ContainerStateRunning = "running"
	ContainerStateStopped = "stopped"

	OwnerKindReplicaSet = "ReplicaSet"
)

type PodInfo struct {
	PodId         string            `json:"podId"`
	TemplateId    string            `json:"templateId,omitempty"`
	OwnerKind     string            `json:"ownerKind,omitempty"`
	OwnerId       string            `json:"ownerId,omitempty"`
	Name          string            `json:"name"`
	Namespace     string            `json:"namespace"`
	UID           string            `json:"uid"`
	State         string            `json:"state"`
	OwnerPid      int               `json:"ownerPid"`
	NetworkNS     string            `json:"networkNS"`
	IPCNS         string            `json:"ipcNS"`
	UTSNS         string            `json:"utsNS"`
	UserNS        string            `json:"userNS"`
	Rootless      bool              `json:"rootless,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	Annotations   map[string]string `json:"annotations,omitempty"`
	CreatedAt     time.Time         `json:"createdAt"`
	StartedAt     time.Time         `json:"startedAt"`
	StoppedAt     time.Time         `json:"stoppedAt"`
	StoppedByUser bool              `json:"stoppedByUser"`
}

type StorePodRequest struct {
	PodId       string
	TemplateId  string
	OwnerKind   string
	OwnerId     string
	Name        string
	Namespace   string
	UID         string
	State       string
	NetworkNS   string
	IPCNS       string
	UTSNS       string
	UserNS      string
	Rootless    bool
	Labels      map[string]string
	Annotations map[string]string
}

type PodTemplateSpec struct {
	Name        string                  `json:"name"`
	Namespace   string                  `json:"namespace"`
	NetworkNS   string                  `json:"networkNS"`
	IPCNS       string                  `json:"ipcNS"`
	UTSNS       string                  `json:"utsNS"`
	UserNS      string                  `json:"userNS"`
	Rootless    bool                    `json:"rootless,omitempty"`
	Labels      map[string]string       `json:"labels,omitempty"`
	Annotations map[string]string       `json:"annotations,omitempty"`
	Containers  []ContainerTemplateSpec `json:"containers,omitempty"`
}

type ContainerTemplateSpec struct {
	Name            string   `json:"name"`
	Image           string   `json:"image"`
	Command         []string `json:"command,omitempty"`
	Port            []string `json:"port,omitempty"`
	Mount           []string `json:"mount,omitempty"`
	Env             []string `json:"env,omitempty"`
	CapAdd          []string `json:"capAdd,omitempty"`
	CapDrop         []string `json:"capDrop,omitempty"`
	SecurityProfile string   `json:"securityProfile,omitempty"`
	Network         string   `json:"network,omitempty"`
	Tty             bool     `json:"tty,omitempty"`
}

type PodTemplateInfo struct {
	TemplateId string          `json:"templateId"`
	Spec       PodTemplateSpec `json:"spec"`
	CreatedAt  time.Time       `json:"createdAt"`
}

type ReplicaSetSpec struct {
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace"`
	Replicas   int               `json:"replicas"`
	TemplateId string            `json:"templateId"`
	Selector   map[string]string `json:"selector,omitempty"`
}

type ReplicaSetInfo struct {
	ReplicaSetId       string         `json:"replicaSetId"`
	Spec               ReplicaSetSpec `json:"spec"`
	CreatedAt          time.Time      `json:"createdAt"`
	ReconcileAttempt   int            `json:"reconcileAttempt,omitempty"`
	LastReconcileError string         `json:"lastReconcileError,omitempty"`
	NextReconcileAt    time.Time      `json:"nextReconcileAt,omitempty"`
}

type DeploymentSpec struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	Replicas     int               `json:"replicas"`
	TemplateId   string            `json:"templateId"`
	Selector     map[string]string `json:"selector,omitempty"`
	ReplicaSetId string            `json:"replicaSetId,omitempty"`
}

type DeploymentInfo struct {
	DeploymentId string         `json:"deploymentId"`
	Spec         DeploymentSpec `json:"spec"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
}

type PodState struct {
	Version      string                     `json:"version"`
	Pods         map[string]PodInfo         `json:"pods"`
	PodTemplates map[string]PodTemplateInfo `json:"podTemplates"`
	ReplicaSets  map[string]ReplicaSetInfo  `json:"replicaSets"`
	Deployments  map[string]DeploymentInfo  `json:"deployments"`
}
