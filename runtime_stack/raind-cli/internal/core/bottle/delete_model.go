package bottle

type ServiceBottleDeleteModel struct {
	Target string
}

type DeleteResponseModel struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
