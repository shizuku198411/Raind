package image

type ServiceImageBuildModel struct {
	ContextDir string
	Tag        string
	Dripfile   string
}

type ImageBuildResponseModel struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
