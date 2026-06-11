package registry

type RegistryPullModel struct {
	Image    string
	Os       string
	Arch     string
	Progress ProgressFunc
}

type ProgressFunc func(ProgressEvent)

type ProgressEvent struct {
	Status  string `json:"status"`
	ID      string `json:"id,omitempty"`
	Detail  string `json:"detail,omitempty"`
	Current int64  `json:"current,omitempty"`
	Total   int64  `json:"total,omitempty"`
	Error   string `json:"error,omitempty"`
}
