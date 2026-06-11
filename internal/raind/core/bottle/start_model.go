package bottle

type ServiceBottleStartModel struct {
	Target string
}

type StartResponseModel struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
