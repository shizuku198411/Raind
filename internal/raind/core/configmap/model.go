package configmap

import "time"

type ConfigMapInfo struct {
	ConfigMapId string            `json:"configMapId"`
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Data        map[string]string `json:"data,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
}

type ListResponseModel struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    []ConfigMapInfo `json:"data,omitempty"`
}

type DetailResponseModel struct {
	Status  string        `json:"status"`
	Message string        `json:"message"`
	Data    ConfigMapInfo `json:"data,omitempty"`
}

type RemoveResponseModel struct {
	Status  string        `json:"status"`
	Message string        `json:"message"`
	Data    ConfigMapInfo `json:"data,omitempty"`
}
