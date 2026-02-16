package network

type NetworkServiceHandler interface {
	CreateNewNetwork(param ServiceNewNetworkModel) (err error)
	RemoveNetwork(param ServiceRemoveNetworkModel) error
	CreateBridgeInterface(ifname string, addr string) error
	CreateMasqueradeRule(src string, dst string) error
	InsertInputRule(num int, ruleModel InputRuleModel, action string) error
	CreateForwardingRule(containerId string, parameter ServiceNetworkModel) error
	CreateRedirectDnsTrafficRule(forwarderIf string, forwarderAddr string) error
	RemoveForwardingRule(containerId string, parameter ServiceNetworkModel) error
}
