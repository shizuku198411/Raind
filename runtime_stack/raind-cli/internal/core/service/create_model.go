package service

type ServiceServiceCreateModel struct {
	FilePath string
}

type CreateResponseDataModel struct {
	ServiceId string `json:"serviceId,omitempty"`
}

type CreateResponseModel struct {
	Status  string                  `json:"status"`
	Message string                  `json:"message"`
	Data    CreateResponseDataModel `json:"data,omitempty"`
}
