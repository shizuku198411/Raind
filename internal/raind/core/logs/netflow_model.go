package logs

type ServiceNetflowModel struct {
	ContainerId string
	TailLine    int
	Pager       bool
	JsonView    bool
}

type NetflowResponseModel struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

type NetflowLog struct {
	GeneratedTS string `json:"generated_ts"`
	ReveivedTS  string `json:"received_ts"`

	Policy Policy `json:"policy"`

	Kind    string `json:"kind"`
	Verdict string `json:"verdict"`
	Proto   string `json:"proto"`

	Icmp *struct {
		Type int `json:"type"`
		Code int `json:"code"`
	}

	Src Endpoint `json:"src"`
	Dst Endpoint `json:"dst"`
}

type Policy struct {
	Source string `json:"source"`
	Id     string `json:"id,omitempty"`
}

type Endpoint struct {
	Kind          string `json:"kind"` // container/external/...
	IP            string `json:"ip"`
	Port          int    `json:"port,omitempty"`
	ContainerId   string `json:"container_id,omitempty"`
	ContainerName string `json:"container_name,omitempty"`
}
