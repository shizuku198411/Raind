package service

import "time"

type ServiceDetailModel struct {
	ServiceId string             `json:"serviceId"`
	Name      string             `json:"name"`
	Namespace string             `json:"namespace"`
	Selector  map[string]string  `json:"selector,omitempty"`
	Ports     []ServicePortModel `json:"ports,omitempty"`
	CreatedAt time.Time          `json:"createdAt"`
}

type DetailResponseModel struct {
	Status  string             `json:"status"`
	Message string             `json:"message"`
	Data    ServiceDetailModel `json:"data,omitempty"`
}
