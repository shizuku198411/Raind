package deployment

type ScaleRequestModel struct {
	Replicas int `json:"replicas"`
}

type ScaleResponseDataModel struct {
	DeploymentId string `json:"deploymentId"`
	Replicas     int    `json:"replicas"`
}

type ScaleResponseModel struct {
	Status  string                 `json:"status"`
	Message string                 `json:"message"`
	Data    ScaleResponseDataModel `json:"data,omitempty"`
}

type ServiceDeploymentScaleModel struct {
	Id       string
	Replicas int
}
