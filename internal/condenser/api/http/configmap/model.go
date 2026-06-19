package configmap

type ConfigMapSummary struct {
	ConfigMapId string            `json:"configMapId"`
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Data        map[string]string `json:"data,omitempty"`
	CreatedAt   string            `json:"createdAt"`
}
