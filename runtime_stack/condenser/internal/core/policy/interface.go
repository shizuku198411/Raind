package policy

type PolicyServiceHandler interface {
	BuildPredefinedPolicy() error
	BuildUserPolicy() error

	GetPolicyList(param ServiceListModel) PolicyListModel

	ChangeNSMode(mode string) error
	AddUserPolicy(param ServiceAddPolicyModel) (string, error)
	RemoveUserPolicy(param ServiceRemovePolicyModel) error

	CommitPolicy() error
	RevertPolicy() error
}

type IptablesHandler interface {
	CreateChain(chainName string) error
	InsertForwardRule(chainName string) error
	AddRuleToChain(chainName string, ruleModel RuleModel, action string) error
	InsertRuleToChain(chainName string, ruleModel RuleModel, action string) error
}
