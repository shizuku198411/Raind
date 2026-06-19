package npm

type NetworkPolicy struct {
	Version     string        `json:"version"`
	DefaultRule DefaultPolicy `json:"defaults"`
	Policies    PolicyChain   `json:"policies"`
}

type DefaultPolicy struct {
	EastWest   PolicyMode `json:"east_west"`
	NorthSouth PolicyMode `json:"north_south"`
}

type PolicyMode struct {
	Mode    string `json:"mode"`
	Logging bool   `json:"logging"`
}

type PolicyChain struct {
	EastWestPolicy          []Policy `json:"east_west"`
	NorthSouthObservePolicy []Policy `json:"north_south_observe"`
	NorthSouthEnforcePolicy []Policy `json:"north_south_enforce"`
}

type Policy struct {
	Id          string   `json:"id"`
	Status      string   `json:"status"`
	Reason      string   `json:"reason,omitempty"`
	Source      HostInfo `json:"source"`
	Destination HostInfo `json:"destination"`
	Protocol    string   `json:"protocol,omitempty"`
	DestPort    int      `json:"dport,omitempty"`
	Comment     string   `json:"comment,omitempty"`
	ManagedBy   string   `json:"managedBy,omitempty"`
	OwnerKind   string   `json:"ownerKind,omitempty"`
	OwnerNS     string   `json:"ownerNamespace,omitempty"`
	OwnerName   string   `json:"ownerName,omitempty"`
	OwnerId     string   `json:"ownerId,omitempty"`
}

type HostInfo struct {
	ContainerName string `json:"container_name,omitempty"`
	DisplayName   string `json:"display_name,omitempty"`
	Address       string `json:"address,omitempty"`
}
