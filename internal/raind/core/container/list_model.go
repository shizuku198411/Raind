package container

import "time"

type ForwardInfoModel struct {
	HostPort      int    `json:"source"`
	ContainerPort int    `json:"destination"`
	Protocol      string `json:"protocol"`
}

type ContainerStateModel struct {
	ContainerId string   `json:"containerId"`
	Name        string   `json:"name"`
	State       string   `json:"state"`
	Pid         int      `json:"pid"`
	Repository  string   `json:"imageRepository"`
	Reference   string   `json:"imageReference"`
	Command     []string `json:"command"`

	Address  string             `json:"address"`
	Forwards []ForwardInfoModel `json:"forwards"`

	CreatingAt time.Time `json:"creatingAt"`
	CreatedAt  time.Time `json:"createdAt"`
	StartedAt  time.Time `json:"statedAt"`
	StoppedAt  time.Time `json:"stoppedAt"`
}

type ListResponseModel struct {
	Status  string                `json:"status"`
	Message string                `json:"message"`
	Data    []ContainerStateModel `json:"data"`
}
