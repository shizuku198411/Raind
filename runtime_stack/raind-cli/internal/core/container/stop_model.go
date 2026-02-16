package container

type ServiceStopModel struct {
	Id string
}

type StopResponseDataModel struct {
	Id string `json:"id"`
}

type StopResponseModel struct {
	Status  string                `json:"status"`
	Message string                `json:"message"`
	Data    StopResponseDataModel `json:"data,omitempty"`
}
