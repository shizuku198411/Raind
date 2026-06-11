package service

import "time"

type ServicePortModel struct {
	Port       int    `json:"port"`
	TargetPort int    `json:"targetPort"`
	Protocol   string `json:"protocol"`
}

type ServiceInfoModel struct {
	ServiceId string             `json:"serviceId"`
	Name      string             `json:"name"`
	Namespace string             `json:"namespace"`
	Ports     []ServicePortModel `json:"ports,omitempty"`
	CreatedAt time.Time          `json:"createdAt"`
}

type ListResponseModel struct {
	Status  string             `json:"status"`
	Message string             `json:"message"`
	Data    []ServiceInfoModel `json:"data,omitempty"`
}
