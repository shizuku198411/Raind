package container

// create options
type CreateOption struct {
	ContainerId  string
	PrintPidFlag bool
	TtyFlag      bool
}

// init options
type InitOption struct {
	ContainerId string
	Fifo        string
	Entrypoint  []string
}

// start options
type StartOption struct {
	ContainerId string
}

// run options
type RunOption struct {
	ContainerId  string
	Tty          bool
	PrintPidFlag bool
}

// exec options
type ExecOption struct {
	ContainerId string
	Tty         bool
	Entrypoint  []string
}

// kill options
type KillOption struct {
	ContainerId string
	Signal      string
}

// delete options
type DeleteOption struct {
	ContainerId string
}

// attach options
type AttachOption struct {
	ContainerId string
}
