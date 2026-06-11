package image

type ServiceImagePullModel struct {
	Image string
	Os    string
	Arch  string
}

type ImagePullRequestModel struct {
	Image string `json:"image"`
	Os    string `json:"os,omitempty"`
	Arch  string `json:"arch,omitempty"`
}

type ImagePullDataModel struct {
	Image string `json:"image"`
	Os    string `json:"os,omitempty"`
	Arch  string `json:"arch,omitempty"`
}

type ImagePullResponseModel struct {
	Status  string             `json:"status"`
	Message string             `json:"message"`
	Data    ImagePullDataModel `json:"data,omitempty"`
}
