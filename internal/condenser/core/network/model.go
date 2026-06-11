package network

type ServiceNewNetworkModel struct {
	Bridge string
}

type ServiceRemoveNetworkModel struct {
	Bridge string
}

type ServiceNetworkModel struct {
	HostPort      string
	ContainerPort string
	Protocol      string
}

type InputRuleModel struct {
	SourceAddr string
	DestAddr   string
	Protocol   string
	SourcePort int
	DestPort   int
}
