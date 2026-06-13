package nsm

import "time"

const DefaultNamespace = "default"
const DefaultNamespaceNetwork = "raind0"

type NamespaceState struct {
	Version   string                   `json:"version"`
	Namespace map[string]NamespaceInfo `json:"namespaces"`
}

type NamespaceInfo struct {
	Name        string            `json:"name"`
	Network     string            `json:"network"`
	NetworkAuto bool              `json:"networkAuto"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
}
