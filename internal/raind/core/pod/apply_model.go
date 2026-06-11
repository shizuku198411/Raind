package pod

type ServicePodApplyModel struct {
	FilePath string
}

type ApplyResponseDataModel struct {
	Pods []PodInfo `json:"pods"`
}

type PodInfo struct {
	PodId        string   `json:"podId"`
	Name         string   `json:"name"`
	Namespace    string   `json:"namespace"`
	ContainerIds []string `json:"containerIds"`
}

type ApplyResponseModel struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Data    ApplyResponseDataModel `json:"data,omitempty"`
}
