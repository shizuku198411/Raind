package service

type CreateServiceRequest struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Selector  map[string]string `json:"selector"`
	Ports     []ServicePort     `json:"ports"`
}

type ServicePort struct {
	Port       int    `json:"port"`
	TargetPort int    `json:"targetPort"`
	Protocol   string `json:"protocol"`
}

type CreateServiceResponse struct {
	ServiceId string `json:"serviceId"`
}

type ServiceSummary struct {
	ServiceId string        `json:"serviceId"`
	Name      string        `json:"name"`
	Namespace string        `json:"namespace"`
	Ports     []ServicePort `json:"ports"`
	CreatedAt string        `json:"createdAt"`
}
