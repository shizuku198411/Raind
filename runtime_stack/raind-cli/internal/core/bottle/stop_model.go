package bottle

type ServiceBottleStopModel struct {
	Target string
}

type StopResponseModel struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
