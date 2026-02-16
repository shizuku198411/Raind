package ssm

import "time"

type ServicePort struct {
	Port       int    `json:"port"`
	TargetPort int    `json:"targetPort"`
	Protocol   string `json:"protocol"`
}

type ServiceInfo struct {
	ServiceId string            `json:"serviceId"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Selector  map[string]string `json:"selector"`
	Ports     []ServicePort     `json:"ports"`
	CreatedAt time.Time         `json:"createdAt"`
}

type ServiceState struct {
	Version  string                 `json:"version"`
	Services map[string]ServiceInfo `json:"services"`
}
