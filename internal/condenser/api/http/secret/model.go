package secret

type SecretSummary struct {
	SecretId  string   `json:"secretId"`
	Name      string   `json:"name"`
	Namespace string   `json:"namespace"`
	Type      string   `json:"type"`
	Keys      []string `json:"keys,omitempty"`
	CreatedAt string   `json:"createdAt"`
}
