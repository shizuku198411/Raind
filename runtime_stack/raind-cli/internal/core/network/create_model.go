package network

type ServiceNetworkCreateModel struct {
	Bridge string
}

type CreateRequestModel struct {
	Bridge string `json:"bridge"`
}

type CreateResponseDataModel struct {
	Bridge string `json:"bridge"`
}

type CreateResponseModel struct {
	Status  string                  `json:"status"`
	Message string                  `json:"message"`
	Data    CreateResponseDataModel `json:"data"`
}
