package policy

type ServiceAddPolicyModel struct {
	ChainName   string
	Source      string
	Destination string
	Protocol    string
	DestPort    int
	Comment     string
}

type ServiceRemovePolicyModel struct {
	Id string
}

type RuleModel struct {
	Conntrack       bool
	Ctstate         []string
	Syn             bool
	Physdev         bool
	PhysdevIsBridge bool
	InputDev        string
	OutputDev       string
	InputPhysdev    string
	OutputPhysdev   string
	Source          string
	Destination     string
	Protocol        string
	SourcePort      int
	DestPort        int

	NflogGroup  int
	NflogPrefix string
}

type ServiceListModel struct {
	Chain string
}

type PolicyListModel struct {
	Mode          string       `json:"mode"`
	PoliciesTotal int          `json:"policies_total"`
	Policies      []PolicyInfo `json:"policies"`
}

type PolicyInfo struct {
	Id          string   `json:"id"`
	Status      string   `json:"status"`
	Reason      string   `json:"reason,omitempty"`
	Source      HostInfo `json:"source"`
	Destination HostInfo `json:"destination"`
	Protocol    string   `json:"protocol,omitempty"`
	DestPort    int      `json:"dport,omitempty"`
	Comment     string   `json:"comment,omitempty"`
}

type HostInfo struct {
	ContainerName string `json:"container_name,omitempty"`
	Address       string `json:"address,omitempty"`
}
