package runtime

type SpecModel struct {
	Rootfs    string
	Cwd       string
	Command   string
	Namespace []string
	NSPath    []string
	Hostname  string
	Env       []string
	Mount     []string

	HostInterface          string
	BridgeInterface        string
	ContainerInterface     string
	ContainerInterfaceAddr string
	ContainerGateway       string
	ContainerDns           []string

	ImageLayer []string
	UpperDir   string
	WorkDir    string

	CreateRuntimeHook      []string
	CreateRuntimeHookEnv   []string
	CreateContainerHook    []string
	CreateContainerHookEnv []string
	StartContainerHook     []string
	StartContainerHookEnv  []string
	PoststartHook          []string
	PoststartHookEnv       []string
	StopContainerHook      []string
	StopContainerHookEnv   []string
	PoststopHook           []string
	PoststopHookEnv        []string

	Output string
}

type CreateModel struct {
	ContainerId string
	Tty         bool
}

type StartModel struct {
	ContainerId string
	Tty         bool
}

type DeleteModel struct {
	ContainerId string
}

type StopModel struct {
	ContainerId string
}

type ExecModel struct {
	ContainerId string
	Entrypoint  []string
	Tty         bool
}
