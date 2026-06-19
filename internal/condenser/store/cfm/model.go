package cfm

import "time"

type ConfigMapInfo struct {
	ConfigMapId string            `json:"configMapId"`
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Data        map[string]string `json:"data,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
}

type ConfigMapState struct {
	Version   string                   `json:"version"`
	ConfigMap map[string]ConfigMapInfo `json:"configMaps"`
}
