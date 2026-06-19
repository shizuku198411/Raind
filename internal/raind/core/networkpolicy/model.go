package networkpolicy

import "time"

type NetworkPolicyInfo struct {
	NetworkPolicyId string            `json:"networkPolicyId"`
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	PodSelector     map[string]string `json:"podSelector,omitempty"`
	IngressRules    int               `json:"ingressRules"`
	EgressRules     int               `json:"egressRules"`
	GeneratedRules  int               `json:"generatedRules"`
	CreatedAt       time.Time         `json:"createdAt"`
}

type ListResponseModel struct {
	Status  string              `json:"status"`
	Message string              `json:"message"`
	Data    []NetworkPolicyInfo `json:"data,omitempty"`
}

type DetailResponseModel struct {
	Status  string            `json:"status"`
	Message string            `json:"message"`
	Data    NetworkPolicyInfo `json:"data,omitempty"`
}

type RemoveResponseModel struct {
	Status  string            `json:"status"`
	Message string            `json:"message"`
	Data    NetworkPolicyInfo `json:"data,omitempty"`
}
