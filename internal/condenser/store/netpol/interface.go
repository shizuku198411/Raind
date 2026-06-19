package netpol

type StoreHandler interface {
	SetNetworkPolicyState() error
}

type Handler interface {
	StoreNetworkPolicy(networkPolicyId string, info NetworkPolicyInfo) error
	GetNetworkPolicyList() ([]NetworkPolicyInfo, error)
	GetNetworkPolicyById(networkPolicyId string) (NetworkPolicyInfo, error)
	GetNetworkPolicyByName(name, namespace string) (NetworkPolicyInfo, error)
	RemoveNetworkPolicy(networkPolicyId string) error
	IsNameAlreadyUsed(name, namespace string) bool
}
