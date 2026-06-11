package policy

type AddPolicyRequest struct {
	ChainName   string `json:"chain" example:"RAIND-EW"`
	Source      string `json:"source" example:"src-container-name"`
	Destination string `json:"dest" example:"dst-container-name"`
	Protocol    string `json:"protocol" example:"tcp"`
	DestPort    int    `json:"dport" example:"443"`
	Comment     string `json:"comment" example:"allow https traffic"`
}

type AddPolicyResponse struct {
	Id string `json:"id"`
}

type ListPolicyRequest struct {
	ChainName string `json:"chain" example:"RAIN-EW"`
}

type ChangeNSModeRequest struct {
	Mode string `json:"mode" example:"enforce"`
}
