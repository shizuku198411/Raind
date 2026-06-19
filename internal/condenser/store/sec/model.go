package sec

import "time"

type SecretInfo struct {
	SecretId  string            `json:"secretId"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Type      string            `json:"type"`
	Data      map[string]string `json:"data,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
}

type SecretState struct {
	Version string                `json:"version"`
	Secrets map[string]SecretInfo `json:"secrets"`
}
