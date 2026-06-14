package ism

import "time"

type IngressBackend struct {
	ServiceName string `json:"serviceName"`
	ServicePort int    `json:"servicePort"`
}

type IngressPath struct {
	Path     string         `json:"path"`
	PathType string         `json:"pathType"`
	Backend  IngressBackend `json:"backend"`
}

type IngressRule struct {
	Host  string        `json:"host"`
	Paths []IngressPath `json:"paths"`
}

type IngressInfo struct {
	IngressId string        `json:"ingressId"`
	Name      string        `json:"name"`
	Namespace string        `json:"namespace"`
	Rules     []IngressRule `json:"rules"`
	TLSHosts  []string      `json:"tlsHosts,omitempty"`
	CreatedAt time.Time     `json:"createdAt"`
}

type IngressState struct {
	Version   string                 `json:"version"`
	Ingresses map[string]IngressInfo `json:"ingresses"`
}
