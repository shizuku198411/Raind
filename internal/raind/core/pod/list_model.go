package pod

import "time"

type PodInfoModel struct {
	PodId             string            `json:"podId"`
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	UID               string            `json:"uid"`
	State             string            `json:"state"`
	NetworkNS         string            `json:"networkNS"`
	IpcNS             string            `json:"ipcNS"`
	UtsNS             string            `json:"utsNS"`
	UserNS            string            `json:"userNS"`
	Labels            map[string]string `json:"labels"`
	Annotations       map[string]string `json:"annotations"`
	CreatedAt         time.Time         `json:"createdAt"`
	StartedAt         time.Time         `json:"startedAt"`
	StoppedAt         time.Time         `json:"stoppedAt"`
	DesiredContainers int               `json:"desiredContainers"`
	RunningContainers int               `json:"runningContainers"`
}

type ListResponseModel struct {
	Status  string         `json:"status"`
	Message string         `json:"message"`
	Data    []PodInfoModel `json:"data"`
}
