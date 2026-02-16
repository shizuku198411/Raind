package pod

type ServicePodRemoveModel struct {
	Id string
}

type RemoveResponseDataModel struct {
	PodId string `json:"podId"`
	Id    string `json:"id,omitempty"`
}

type RemoveResponseModel struct {
	Status  string                  `json:"status"`
	Message string                  `json:"message"`
	Data    RemoveResponseDataModel `json:"data,omitempty"`
}
