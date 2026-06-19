package netpol

import "time"

type NetworkPolicyInfo struct {
	NetworkPolicyId  string            `json:"networkPolicyId"`
	Name             string            `json:"name"`
	Namespace        string            `json:"namespace"`
	PodSelector      map[string]string `json:"podSelector,omitempty"`
	Ingress          []RuleInfo        `json:"ingress,omitempty"`
	Egress           []RuleInfo        `json:"egress,omitempty"`
	GeneratedRuleIds []string          `json:"generatedRuleIds,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
}

type RuleInfo struct {
	Direction   string            `json:"direction"`
	PodSelector map[string]string `json:"podSelector,omitempty"`
	Protocol    string            `json:"protocol,omitempty"`
	Port        int               `json:"port,omitempty"`
}

type NetworkPolicyState struct {
	Version         string                       `json:"version"`
	NetworkPolicies map[string]NetworkPolicyInfo `json:"networkPolicies"`
}
