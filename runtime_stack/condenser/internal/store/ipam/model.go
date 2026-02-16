package ipam

import (
	"time"
)

type ForwardInfo struct {
	HostPort      int    `json:"source"`
	ContainerPort int    `json:"destination"`
	Protocol      string `json:"protocol"`
}

type Allocation struct {
	ContainerId string        `json:"containerId"`
	Interface   string        `json:"interface"`
	Forwards    []ForwardInfo `json:"forwards"`
	AssignedAt  time.Time     `json:"assignedAt"`
}

type Pool struct {
	Interface   string                `json:"interface"`
	Subnet      string                `json:"subnet"`
	Address     string                `json:"address"`
	Allocations map[string]Allocation `json:"allocations"`
}

type DnsProxy struct {
	DnsProxyInterface string   `json:"dnsProxyInterface"`
	DnsProxyAddr      string   `json:"dnsProxyAddr"`
	Upstreams         []string `json:"upstreams"`
}

type IpamState struct {
	Version           string   `json:"version"`
	RuntimeSubnet     string   `json:"runtimeSubnet"`
	DnsProxy          DnsProxy `json:"dnsProxy"`
	HostInterface     string   `json:"hostInterface"`
	HostInterfaceAddr string   `json:"hostInterfaceAddress"`
	Pools             []Pool   `json:"pools"`
}

type NetworkList struct {
	Interface     string
	Address       string
	NumContainers int
}
