package ipam

type IpamStoreHandler interface {
	SetConfig() error
}

type IpamHandler interface {
	Allocate(containerId string, bridge string) (string, error)
	Release(containerId string) error
	GetNetworkList() ([]NetworkList, error)
	StoreBridge(bridge string) (string, string, error)
	RemoveBridge(bridge string) error
	GetRuntimeSubnet() (string, error)
	GetDefaultInterface() (string, error)
	GetDefaultInterfaceAddr() (string, error)
	GetBridgeAddr(bridgeInterface string) (string, error)
	GetDnsProxyInfo() (string, string, []string, error)
	GetContainerAddress(containerId string) (string, string, string, error)
	GetInfoByIp(ip string) (string, string, error)
	SetForwardInfo(containerId string, sport, dport int, protocol string) error
	GetForwardInfo(containerId string) ([]ForwardInfo, error)
	GetPoolList() ([]Pool, error)
	GetNetworkInfoById(containerId string) (string, Allocation, error)
	GetVethById(containerId string) (string, error)
}
