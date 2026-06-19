package networkpolicy

type NetworkPolicySummary struct {
	NetworkPolicyId string            `json:"networkPolicyId"`
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	PodSelector     map[string]string `json:"podSelector,omitempty"`
	IngressRules    int               `json:"ingressRules"`
	EgressRules     int               `json:"egressRules"`
	GeneratedRules  int               `json:"generatedRules"`
	CreatedAt       string            `json:"createdAt"`
}
