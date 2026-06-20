package promote

type ContainerToBottleOptions struct {
	BottleName        string
	ServiceName       string
	IncludeImageEnv   bool
	AllowPodContainer bool
}

type BottleDraft struct {
	SourceContainer string
	BottleName      string
	Services        []ServiceDraft

	// Legacy single-service fields are kept for tests and callers that
	// inspect the draft returned by BuildBottleDraftFromContainer.
	ServiceName string
	Image       string
	Command     []string
	Env         []EnvVar
	Ports       []PortMapping
	Mounts      []MountMapping
	Network     string
	Tty         bool
	Warnings    []Warning
}

type ServiceDraft struct {
	Name      string
	Image     string
	Command   []string
	Env       []EnvVar
	Ports     []PortMapping
	Mounts    []MountMapping
	Network   string
	Tty       bool
	DependsOn []string
}

type EnvVar struct {
	Key       string
	Value     string
	Sensitive bool
}

type PortMapping struct {
	HostPort      int
	ContainerPort int
	Protocol      string
}

type MountMapping struct {
	Source      string
	Destination string
	ReadOnly    bool
	Options     []string
}

type Warning struct {
	Code    string
	Message string
}
