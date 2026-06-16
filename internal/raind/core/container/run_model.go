package container

type ServiceRunModel struct {
	Image           string
	Command         []string
	Network         string
	Volume          []string
	Publish         []string
	Device          []string
	Env             []string
	CapAdd          []string
	CapDrop         []string
	SecurityProfile string
	Tty             bool
	Rm              bool
	Name            string
	Rootless        bool
	RootlessMode    string
	RootlessRootUID int
	RootlessRootGID int
}
