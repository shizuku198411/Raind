package service

type RemoveResponseDataModel struct {
	ServiceId string `json:"serviceId,omitempty"`
}

type RemoveResponseModel struct {
	Status  string                  `json:"status"`
	Message string                  `json:"message"`
	Data    RemoveResponseDataModel `json:"data,omitempty"`
}

type ServiceServiceRemoveModel struct {
	Id string
}
