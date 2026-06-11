package image

import "time"

type PullImageRequest struct {
	Image string `json:"image" example:"alpine:latest"`
	Os    string `json:"os" example:"linux"`
	Arch  string `json:"arch" example:"arm64"`
}

type RemoveImageRequest struct {
	Image string `json:"image" example:"alpine:latest"`
}

// == build ==
type BuildImageResponse struct {
	Image string `json:"image"`
}

// == status ==
type ImageStatusResponse struct {
	Repository  string    `json:"repository"`
	Reference   string    `json:"reference"`
	Id          string    `json:"id"`
	RepoTags    []string  `json:"repoTags"`
	RepoDigests []string  `json:"repoDigests"`
	SizeBytes   int64     `json:"sizeBytes"`
	CreatedAt   time.Time `json:"createdAt"`
	User        string    `json:"user"`
}

// == fs info ==
type ImageFsInfoResponse struct {
	Image     string `json:"image"`
	UsedBytes int64  `json:"usedBytes"`
}
