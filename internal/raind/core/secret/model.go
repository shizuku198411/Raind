package secret

import "time"

type SecretInfo struct {
	SecretId  string    `json:"secretId"`
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	Type      string    `json:"type"`
	Keys      []string  `json:"keys,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type ListResponseModel struct {
	Status  string       `json:"status"`
	Message string       `json:"message"`
	Data    []SecretInfo `json:"data,omitempty"`
}

type DetailResponseModel struct {
	Status  string     `json:"status"`
	Message string     `json:"message"`
	Data    SecretInfo `json:"data,omitempty"`
}

type RemoveResponseModel struct {
	Status  string     `json:"status"`
	Message string     `json:"message"`
	Data    SecretInfo `json:"data,omitempty"`
}
