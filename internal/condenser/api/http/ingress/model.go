package ingress

import "raind/internal/condenser/store/ism"

type IngressSummary struct {
	IngressId string            `json:"ingressId"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Rules     []ism.IngressRule `json:"rules"`
	TLSHosts  []string          `json:"tlsHosts,omitempty"`
	CreatedAt string            `json:"createdAt"`
}
