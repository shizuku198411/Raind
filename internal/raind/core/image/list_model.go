package image

import "time"

type ImageDataModel struct {
	Repository string    `json:"repository"`
	Reference  string    `json:"reference"`
	CreatedAt  time.Time `json:"createdAt"`
}

type ImageListResponseModel struct {
	Status  string           `json:"status"`
	Message string           `json:"message"`
	Data    []ImageDataModel `json:"data"`
}
