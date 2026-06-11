package ilm

import "time"

type ReferenceInfo struct {
	BundlePath string    `json:"bundlePath"`
	ConfigPath string    `json:"configPath"`
	RootfsPath string    `json:"rootfsPath"`
	CreatedAt  time.Time `json:"createdAt"`
}

type RepositoryInfo struct {
	References map[string]ReferenceInfo `json:"references"`
}

type ImageLayerState struct {
	Version      string                    `json:"version"`
	Repositories map[string]RepositoryInfo `json:"repositories"`
}

type ImageInfo struct {
	Repository string
	Reference  string
	CreatedAt  time.Time
}
