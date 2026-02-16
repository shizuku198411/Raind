package bottle

type BottleSpec struct {
	Bottle   BottleMeta             `yaml:"bottle"`
	Services map[string]ServiceSpec `yaml:"services"`
	Policies []PolicySpec           `yaml:"policies,omitempty"`
}

type BottleMeta struct {
	Name string `yaml:"name"`
}

type ServiceSpec struct {
	Image     string   `yaml:"image"`
	Command   []string `yaml:"command,omitempty"`
	Env       []string `yaml:"env,omitempty"`
	Ports     []string `yaml:"ports,omitempty"`
	Mount     []string `yaml:"mount,omitempty"`
	Network   string   `yaml:"network,omitempty"`
	Tty       bool     `yaml:"tty,omitempty"`
	DependsOn []string `yaml:"depends_on,omitempty"`
}

type PolicySpec struct {
	Type        string `yaml:"type"`
	Source      string `yaml:"source"`
	Destination string `yaml:"destination"`
	Protocol    string `yaml:"protocol,omitempty"`
	DestPort    int    `yaml:"dest_port,omitempty"`
	Comment     string `yaml:"comment,omitempty"`
}
