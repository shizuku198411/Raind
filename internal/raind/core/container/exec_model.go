package container

type ServiceExecModel struct {
	ContainerId string
	Tty         bool
	Command     []string
}

type ExecRequestModel struct {
	Command []string `json:"command"`
	Tty     bool     `json:"tty"`
}

type ExecResponseModel struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
