package container

type ServiceStartModel struct {
	Id  string
	Tty bool
}

type StartRequestModel struct {
	Tty bool `json:"tty"`
}

type StartResponseDataModel struct {
	Id string `json:"id"`
}

type StartResponseModel struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Data    StartResponseDataModel `json:"data,omitempty"`
}
