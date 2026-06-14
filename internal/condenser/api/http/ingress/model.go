package ingress

import "raind/internal/condenser/store/ism"

type IngressSummary struct {
	IngressId string            `json:"ingressId"`
	Name      string            `json:"name"`
	Namespace string            `json:"namespace"`
	Rules     []ism.IngressRule `json:"rules"`
	CreatedAt string            `json:"createdAt"`
}
