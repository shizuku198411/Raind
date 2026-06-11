package bottle

type BottleListItemModel struct {
	BottleId     string `json:"bottleId"`
	BottleName   string `json:"bottleName"`
	ServiceCount int    `json:"serviceCount"`
	Status       string `json:"status"`
}

type BottleListDataModel struct {
	Bottles []BottleListItemModel `json:"bottles"`
}

type ListResponseModel struct {
	Status  string              `json:"status"`
	Message string              `json:"message"`
	Data    BottleListDataModel `json:"data"`
}
