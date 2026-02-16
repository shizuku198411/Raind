package bottle

type ServiceBottleCreateModel struct {
	Yaml []byte
}

type CreateResponseDataModel struct {
	BottleId   string   `json:"bottleId"`
	BottleName string   `json:"bottleName"`
	Services   []string `json:"services"`
	StartOrder []string `json:"startOrder"`
}

type CreateResponseModel struct {
	Status  string                  `json:"status"`
	Message string                  `json:"message"`
	Data    CreateResponseDataModel `json:"data,omitempty"`
}
