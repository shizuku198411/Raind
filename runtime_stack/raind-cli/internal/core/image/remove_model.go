package image

type ServiceImageRemoveModel struct {
	Image string
}

type ImageRemoveRequestModel struct {
	Image string `json:"image"`
}

type ImageRemoveDataModel struct {
	Image string `json:"image"`
}

type ImageRemoveResponseModel struct {
	Status  string               `json:"status"`
	Message string               `json:"message"`
	Data    ImageRemoveDataModel `json:"data"`
}
