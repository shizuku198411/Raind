package ingress

import "time"

type IngressBackendModel struct {
	ServiceName string `json:"serviceName"`
	ServicePort int    `json:"servicePort"`
}

type IngressPathModel struct {
	Path     string              `json:"path"`
	PathType string              `json:"pathType"`
	Backend  IngressBackendModel `json:"backend"`
}

type IngressRuleModel struct {
	Host  string             `json:"host"`
	Paths []IngressPathModel `json:"paths"`
}

type IngressInfoModel struct {
	IngressId string             `json:"ingressId"`
	Name      string             `json:"name"`
	Namespace string             `json:"namespace"`
	Rules     []IngressRuleModel `json:"rules,omitempty"`
	CreatedAt time.Time          `json:"createdAt"`
}

type ListResponseModel struct {
	Status  string             `json:"status"`
	Message string             `json:"message"`
	Data    []IngressInfoModel `json:"data,omitempty"`
}
