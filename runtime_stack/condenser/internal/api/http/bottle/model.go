package bottle

type RegisterBottleResponse struct {
	BottleId   string   `json:"bottleId"`
	BottleName string   `json:"bottleName"`
	Services   []string `json:"services"`
	StartOrder []string `json:"startOrder"`
}

type ActionBottleResponse struct {
	Id string `json:"id"`
}

type BottleSummary struct {
	BottleId     string `json:"bottleId"`
	BottleName   string `json:"bottleName"`
	ServiceCount int    `json:"serviceCount"`
	Status       string `json:"status"`
}

type GetBottleListResponse struct {
	Bottles []BottleSummary `json:"bottles"`
}

type BottleDetail struct {
	BottleId   string                       `json:"bottleId"`
	BottleName string                       `json:"bottleName"`
	Services   map[string]BottleServiceSpec `json:"services"`
	StartOrder []string                     `json:"startOrder"`
	Containers map[string]BottleContainerState `json:"containers"`
	Policies   []BottlePolicyInfo           `json:"policies,omitempty"`
	Network    string                       `json:"network,omitempty"`
	NetworkAuto bool                         `json:"networkAuto,omitempty"`
	CreatedAt  string                       `json:"createdAt"`
}

type BottleServiceSpec struct {
	Image     string   `json:"image"`
	Command   []string `json:"command,omitempty"`
	Env       []string `json:"env,omitempty"`
	Ports     []string `json:"ports,omitempty"`
	Mount     []string `json:"mount,omitempty"`
	Network   string   `json:"network,omitempty"`
	Tty       bool     `json:"tty,omitempty"`
	DependsOn []string `json:"dependsOn,omitempty"`
}

type BottlePolicyInfo struct {
	Id          string `json:"id"`
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Protocol    string `json:"protocol,omitempty"`
	DestPort    int    `json:"destPort,omitempty"`
	Comment     string `json:"comment,omitempty"`
}

type BottleContainerState struct {
	ContainerId string               `json:"containerId"`
	Name        string               `json:"name"`
	State       string               `json:"state"`
	Pid         int                  `json:"pid"`
	Repository  string               `json:"imageRepository"`
	Reference   string               `json:"imageReference"`
	Command     []string             `json:"command"`
	Address     string               `json:"address"`
	Forwards    []BottleForwardInfo  `json:"forwards"`
	CreatingAt  string               `json:"creatingAt"`
	CreatedAt   string               `json:"createdAt"`
	StartedAt   string               `json:"statedAt"`
	StoppedAt   string               `json:"stoppedAt"`
}

type BottleForwardInfo struct {
	HostPort      int    `json:"source"`
	ContainerPort int    `json:"destination"`
	Protocol      string `json:"protocol"`
}

type GetBottleResponse struct {
	Bottle BottleDetail `json:"bottle"`
}
