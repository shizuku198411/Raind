package policy

type RemoveRequestModel struct {
	Id string
}

type RemoveResponseModel struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
