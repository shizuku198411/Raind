package npm

type NpmStoreHandler interface {
	SetNetworkPolicy() error
	Backup() error
	Revert() (err error)
}

type NpmHandler interface {
	GetEWMode() string
	GetEWLogging() bool
	GetNSMode() string
	GetNSLogging() bool
	IsNsEnforce() bool

	GetEWPolicyList() []Policy
	GetNSObsPolicyList() []Policy
	GetNSEnfPolicyList() []Policy
	GetPolicyChain(policyId string) (string, error)

	AddPolicy(chainName string, policy Policy) error
	RemovePolicy(policyId string) error

	UpdateStatus(chainName string, ruleId string, status string, reason string) error
	ChangeNSMode(mode string) error
}
