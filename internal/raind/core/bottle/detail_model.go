package bottle

import "raind/internal/raind/core/container"

type BottleServiceModel struct {
	Image     string   `json:"image"`
	Command   []string `json:"command"`
	Env       []string `json:"env"`
	Ports     []string `json:"ports"`
	Mount     []string `json:"mount"`
	CapAdd    []string `json:"capAdd"`
	CapDrop   []string `json:"capDrop"`
	Network   string   `json:"network"`
	Tty       bool     `json:"tty"`
	DependsOn []string `json:"dependsOn"`
}

type BottlePolicyModel struct {
	Id          string `json:"id"`
	Type        string `json:"type"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Protocol    string `json:"protocol"`
	DestPort    int    `json:"destPort"`
	Comment     string `json:"comment"`
}

type BottleDetailModel struct {
	BottleId   string                                   `json:"bottleId"`
	BottleName string                                   `json:"bottleName"`
	Services   map[string]BottleServiceModel            `json:"services"`
	StartOrder []string                                 `json:"startOrder"`
	Containers map[string]container.ContainerStateModel `json:"containers"`
	Policies   []BottlePolicyModel                      `json:"policies"`
	CreatedAt  string                                   `json:"createdAt"`
}

type DetailResponseDataModel struct {
	Bottle BottleDetailModel `json:"bottle"`
}

type DetailResponseModel struct {
	Status  string                  `json:"status"`
	Message string                  `json:"message"`
	Data    DetailResponseDataModel `json:"data"`
}
