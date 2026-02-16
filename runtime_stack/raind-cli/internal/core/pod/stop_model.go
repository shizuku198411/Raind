package pod

type ServicePodStopModel struct {
	Id string
}

type StopResponseDataModel struct {
	PodId string `json:"podId"`
	Id    string `json:"id,omitempty"`
}

type StopResponseModel struct {
	Status  string                `json:"status"`
	Message string                `json:"message"`
	Data    StopResponseDataModel `json:"data,omitempty"`
}
