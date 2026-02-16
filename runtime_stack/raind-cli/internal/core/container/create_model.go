package container

type ServiceCreateModel struct {
	Image   string
	Command []string
	Network string
	Volume  []string
	Publish []string
	Env     []string
	Tty     bool
	Name    string
	PodId   string
}

type CreateRequestModel struct {
	Image   string   `json:"image,omitempty"`
	Command []string `json:"command,omitempty"`
	Network string   `json:"network,omitempty"`
	Volume  []string `json:"mount,omitempty"`
	Publish []string `json:"port,omitempty"`
	Env     []string `json:"env,omitempty"`
	Tty     bool     `json:"tty,omitempty"`
	Name    string   `json:"name,omitempty"`
	PodId   string   `json:"podId,omitempty"`
}

type CreateResponseDataModel struct {
	Id string `json:"id"`
}

type CreateResponseModel struct {
	Status  string                  `json:"status"`
	Message string                  `json:"message"`
	Data    CreateResponseDataModel `json:"data,omitempty"`
}
