package image

import "time"

type ServicePullModel struct {
	Image string
	Os    string
	Arch  string
}

type ServiceRemoveModel struct {
	Image string
}

type ServiceBuildModel struct {
	Image        string
	ContextDir   string
	DripfilePath string
	Network      string
}

// image bundle object
type ImageConfigObject struct {
	Env        []string `json:"Env"`
	Cmd        []string `json:"Cmd"`
	Entrypoint []string `json:"Entrypoint"`
	WorkingDir string   `json:"WorkingDir"`
	User       string   `json:"User"`
}

type ImageConfigFile struct {
	Config ImageConfigObject `json:"config"`
}

type ImageInfo struct {
	Repository string    `json:"repository"`
	Reference  string    `json:"reference"`
	CreatedAt  time.Time `json:"createdAt"`
}

type ImageStatusInfo struct {
	Repository  string    `json:"repository"`
	Reference   string    `json:"reference"`
	Id          string    `json:"id"`
	RepoTags    []string  `json:"repoTags"`
	RepoDigests []string  `json:"repoDigests"`
	SizeBytes   int64     `json:"sizeBytes"`
	CreatedAt   time.Time `json:"createdAt"`
	User        string    `json:"user"`
}

type ImageFsInfo struct {
	Image     string `json:"image"`
	UsedBytes int64  `json:"usedBytes"`
}
