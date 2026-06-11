package container

type ServiceRemoveModel struct {
	Id string
}

type RemoveResponseDataModel struct {
	Id string `json:"id"`
}

type RemoveResponseModel struct {
	Status  string                  `json:"status"`
	Message string                  `json:"message"`
	Data    RemoveResponseDataModel `json:"data,omitempty"`
}
