package policy

type ServiceCreateModel struct {
	Chain       string
	Source      string
	Destination string
	Protocol    string
	DestPort    int
	Comment     string
}

type CreateRequestModel struct {
	Chain       string `json:"chain"`
	Source      string `json:"source"`
	Destination string `json:"dest"`
	Protocol    string `json:"protocol"`
	DestPort    int    `json:"dport"`
	Comment     string `json:"comment"`
}

type ResponseDataModel struct {
	Id string `json:"id"`
}

type CreateResponseModel struct {
	Status  string            `json:"status"`
	Message string            `json:"message"`
	Data    ResponseDataModel `json:"data"`
}
