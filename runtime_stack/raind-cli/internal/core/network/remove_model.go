package network

type ServiceNetworkRemoveModel struct {
	Bridge string
}

type RemoveResponseModel struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
