package deployment

type RemoveResponseDataModel struct {
	DeploymentId string `json:"deploymentId,omitempty"`
}

type RemoveResponseModel struct {
	Status  string                  `json:"status"`
	Message string                  `json:"message"`
	Data    RemoveResponseDataModel `json:"data,omitempty"`
}

type ServiceDeploymentRemoveModel struct {
	Id string
}
