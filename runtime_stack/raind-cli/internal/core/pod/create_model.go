package pod

type ServicePodCreateModel struct {
	Name        string
	Namespace   string
	UID         string
	Labels      map[string]string
	Annotations map[string]string
}

type CreateRequestModel struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace,omitempty"`
	UID         string            `json:"uid,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type CreateResponseDataModel struct {
	PodId string `json:"podId,omitempty"`
	Id    string `json:"id,omitempty"`
}

type CreateResponseModel struct {
	Status  string                  `json:"status"`
	Message string                  `json:"message"`
	Data    CreateResponseDataModel `json:"data,omitempty"`
}
