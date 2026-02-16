package network

type NetworkInfoModel struct {
	Interface     string
	Address       string
	NumContainers int
}

type ListResponseModel struct {
	Status  string             `json:"status"`
	Message string             `json:"message"`
	Data    []NetworkInfoModel `json:"data"`
}
