package policy

type ServiceChangeModeModel struct {
	Mode string
}

type ChangeModeRequestModel struct {
	Mode string `json:"mode"`
}

type ChangeModeResponseModel struct {
	Status  string         `json:"status"`
	Message string         `json:"message"`
	Data    ChangeModeData `json:"data"`
}

type ChangeModeData struct {
	Mode string `json:"mode"`
}
