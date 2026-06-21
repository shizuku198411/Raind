package container

import "time"

type ContainerInspectModel struct {
	ContainerId     string             `json:"containerId"`
	Name            string             `json:"name"`
	PodId           string             `json:"podId,omitempty"`
	DropletId       string             `json:"dropletId,omitempty"`
	State           string             `json:"state"`
	Pid             int                `json:"pid"`
	ImageRepository string             `json:"imageRepository"`
	ImageReference  string             `json:"imageReference"`
	Command         []string           `json:"command"`
	Address         string             `json:"address,omitempty"`
	Forwards        []ForwardInfoModel `json:"forwards,omitempty"`
	SecurityProfile string             `json:"securityProfile"`
	LogPath         string             `json:"logPath"`
	Tty             bool               `json:"tty"`
	CreatedAt       time.Time          `json:"createdAt"`
	StartedAt       time.Time          `json:"startedAt"`
	StoppedAt       time.Time          `json:"stoppedAt"`
	Config          map[string]any     `json:"config"`
}

type InspectResponseDataModel struct {
	Container ContainerInspectModel `json:"container"`
}

type InspectResponseModel struct {
	Status  string                   `json:"status"`
	Message string                   `json:"message"`
	Data    InspectResponseDataModel `json:"data"`
}
