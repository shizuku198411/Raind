package dns

type DnsEvent struct {
	Ts        string `json:"generated_ts"`
	EventType string `json:"event_type"`

	Network Network `json:"network"`
	Client  Client  `json:"src"`

	Dns      DnsBlock      `json:"dns"`
	Upstream *UpstreamInfo `json:"upstream,omitempty"`

	LatencyMs int64      `json:"latency_ms,omitempty"`
	Cache     *CacheInfo `json:"cache,omitempty"`

	Result string `json:"query_result"`
	Note   string `json:"note,omitempty"`
}

type Network struct {
	Transport string `json:"transport"`
}

type Client struct {
	Ip            string `json:"ip,omitempty"`
	Port          int    `json:"port,omitempty"`
	ContainerId   string `json:"container_id,omitempty"`
	ContainerName string `json:"container_name,omitempty"`
	SpiffeId      string `json:"spiffe_id,omitempty"`
	Veth          string `json:"veth,omitempty"`
}

type DnsBlock struct {
	Id       uint16       `json:"id,omitempty"`
	Rd       bool         `json:"rd,omitempty"`
	Question DnsQuestion  `json:"question"`
	Response *DnsResponse `json:"response,omitempty"`
}

type DnsQuestion struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Class string `json:"class"`
}

type DnsResponse struct {
	Rcode      string `json:"rcode"`
	Answers    int    `json:"answers"`
	Authority  int    `json:"authority"`
	Additional int    `json:"additional"`
	Truncated  bool   `json:"truncated"`
}

type UpstreamInfo struct {
	Server    string `json:"server"`
	Transport string `json:"transport"`
}

type CacheInfo struct {
	Hit bool `json:"hit"`
}
