package container

type ServiceRunModel struct {
	Image   string
	Command []string
	Network string
	Volume  []string
	Publish []string
	Env     []string
	Tty     bool
	Rm      bool
	Name    string
}
