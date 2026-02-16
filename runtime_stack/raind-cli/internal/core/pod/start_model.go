package pod

type ServicePodStartModel struct {
	Id string
}

type StartResponseDataModel struct {
	PodId string `json:"podId"`
	Id    string `json:"id,omitempty"`
}

type StartResponseModel struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Data    StartResponseDataModel `json:"data,omitempty"`
}
