package enrichedlog

type ContainerMeta struct {
	ContainerId   string `json:"container_id"`
	ContainerName string `json:"container_name"`
	Ipv4          string `json:"ip"`
	Veth          string `json:"veth,omitempty"`
	SpiffeId      string `json:"spiffe_id,omitempty"`
}

type Endpoint struct {
	Kind          string `json:"kind"`
	Ip            string `json:"ip,omitempty"`
	Port          int    `json:"port,omitempty"`
	ContainerId   string `json:"container_id,omitempty"`
	ContainerName string `json:"container_name,omitempty"`
	SpiffeId      string `json:"spiffe_id,omitempty"`
	Veth          string `json:"veth,omitempty"`
}

type Policy struct {
	Source string `json:"source"`
	Id     string `json:"id,omitempty"`
}

type Enriched struct {
	GeneratedTS string         `json:"generated_ts"`
	ReceivedTS  string         `json:"received_ts"`
	EventType   string         `json:"event_type"`
	Policy      Policy         `json:"policy"`
	Kind        string         `json:"kind"`
	Verdict     string         `json:"verdict"`
	Proto       string         `json:"proto"`
	Src         Endpoint       `json:"src"`
	Dst         Endpoint       `json:"dst"`
	ICMP        map[string]int `json:"icmp,omitempty"`
	RuleHint    string         `json:"rule_hint,omitempty"`
	RawHash     string         `json:"raw_hash"`
	Unresolved  bool           `json:"unresolved,omitempty"`
	Reason      string         `json:"reason,omitempty"`
}
