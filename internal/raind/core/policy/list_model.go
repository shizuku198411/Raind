package policy

type ListRequestModel struct {
	ChainName string
}

type ListResponseModel struct {
	Status  string        `json:"status"`
	Message string        `json:"message"`
	Data    ListDataModel `json:"data"`
}

type ListDataModel struct {
	Mode          string        `json:"mode"`
	PoliciesTotal int           `json:"policies_total"`
	Policies      []PolicyModel `json:"policies"`
}

type PolicyModel struct {
	Id          string        `json:"id"`
	Status      string        `json:"status"`
	Reason      string        `json:"reason"`
	Source      HostInfoModel `json:"source"`
	Destination HostInfoModel `json:"destination"`
	Protocol    string        `json:"protocol"`
	DestPort    int           `json:"dport"`
	Comment     string        `json:"comment"`
}

type HostInfoModel struct {
	ContainerName string `json:"container_name"`
	Address       string `json:"address"`
}
