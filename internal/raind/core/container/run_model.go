package container

type ServiceRunModel struct {
	Image   string
	Command []string
	Network string
	Volume  []string
	Publish []string
	Device  []string
	Env     []string
	CapAdd  []string
	CapDrop []string
	Tty     bool
	Rm      bool
	Name    string
}
