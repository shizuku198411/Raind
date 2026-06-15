package spec

type MountOption struct {
	Destination string
	Type        string
	Source      string
	Options     []string
}

type ProcessOption struct {
	Cwd     string
	Env     []string
	Args    []string
	CapAdd  []string
	CapDrop []string
}

type NamespaceOption struct {
	Type string
	Path string
}

type NetOption struct {
	HostInterface       string
	BridgeInterfaceName string
	InterfaceName       string
	Address             string
	Gateway             string
	Dns                 []string
}

type ImageOption struct {
	ImageLayer []string
	UpperDir   string
	WorkDir    string
}

type HookOption struct {
	Path    string
	Args    []string
	Env     []string
	Timeout *int
}

type HookLifecycleOption struct {
	Prestart        []HookOption
	CreateRuntime   []HookOption
	CreateContainer []HookOption
	StartContainer  []HookOption
	Poststart       []HookOption
	StopContainer   []HookOption
	Poststop        []HookOption
}

type ConfigOptions struct {
	Rootfs          string
	Mounts          []MountOption
	Process         ProcessOption
	Namespace       []NamespaceOption
	Hostname        string
	Net             NetOption
	Image           ImageOption
	Hooks           HookLifecycleOption
	Rootless        bool
	RootlessMode    string
	RootlessRootUID int
	RootlessRootGID int
}
