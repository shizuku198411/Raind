package pod

import (
	"condenser/internal/store/psm"
	"time"
)

type ServiceCreateModel struct {
	Name        string
	Namespace   string
	UID         string
	NetworkNS   string
	IPCNS       string
	UTSNS       string
	UserNS      string
	Labels      map[string]string
	Annotations map[string]string
	Containers  []psm.ContainerTemplateSpec
}

type PodState struct {
	PodId             string            `json:"podId"`
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	UID               string            `json:"uid"`
	State             string            `json:"state"`
	DesiredContainers int               `json:"desiredContainers"`
	RunningContainers int               `json:"runningContainers"`
	NetworkNS         string            `json:"networkNS"`
	IPCNS             string            `json:"ipcNS"`
	UTSNS             string            `json:"utsNS"`
	UserNS            string            `json:"userNS"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	CreatedAt         time.Time         `json:"createdAt"`
	StartedAt         time.Time         `json:"startedAt"`
	StoppedAt         time.Time         `json:"stoppedAt"`
}
