package ipam

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"time"
)

func NewIpamManager(ipamStore *IpamStore) *IpamManager {
	return &IpamManager{
		ipamStore: ipamStore,
	}
}

type IpamManager struct {
	ipamStore *IpamStore
}

func (m *IpamManager) Allocate(containerId string, bridge string) (string, error) {
	var allocated string

	err := m.ipamStore.withLock(func(st *IpamState) error {
		for _, p := range st.Pools {
			if p.Interface == bridge {
				if p.Subnet == "" || p.Address == "" {
					return fmt.Errorf("ipam not configured")
				}
				_, ipnet, _ := net.ParseCIDR(p.Subnet)
				gw := net.ParseIP(strings.Split(p.Address, "/")[0]).To4()
				if gw == nil {
					return fmt.Errorf("gateway must be ipv4")
				}
				next, err := findFreeIpv4(ipnet, gw, p.Allocations)
				if err != nil {
					return err
				}
				ipStr := next.String()
				p.Allocations[ipStr] = Allocation{
					ContainerId: containerId,
					Interface:   "rd_" + containerId,
					AssignedAt:  time.Now(),
				}
				allocated = ipStr
				return nil
			}
		}
		return fmt.Errorf("target bridge not configured: %s", bridge)
	})
	return allocated, err
}

func (m *IpamManager) Release(containerId string) error {
	return m.ipamStore.withLock(func(st *IpamState) error {
		for _, p := range st.Pools {
			for ip, a := range p.Allocations {
				if a.ContainerId == containerId {
					delete(p.Allocations, ip)
					return nil
				}
			}
		}
		return nil
	})
}

func (m *IpamManager) GetNetworkList() ([]NetworkList, error) {
	var networkList []NetworkList

	err := m.ipamStore.withRLock(func(st *IpamState) error {
		for _, p := range st.Pools {
			networkList = append(networkList, NetworkList{
				Interface:     p.Interface,
				Address:       p.Address,
				NumContainers: len(p.Allocations),
			})
		}
		if len(networkList) == 0 {
			return fmt.Errorf("network is not configured")
		}
		return nil
	})
	return networkList, err
}

func (m *IpamManager) GetRuntimeSubnet() (string, error) {
	var runtimeSubnet string

	err := m.ipamStore.withRLock(func(st *IpamState) error {
		runtimeSubnet = st.RuntimeSubnet
		if runtimeSubnet == "" {
			return fmt.Errorf("runtime subnet is not configured")
		}
		return nil
	})
	return runtimeSubnet, err
}

func (m *IpamManager) GetDefaultInterface() (string, error) {
	var defaultInterface string

	err := m.ipamStore.withRLock(func(st *IpamState) error {
		defaultInterface = st.HostInterface
		if defaultInterface == "" {
			return fmt.Errorf("default interface is not configured")
		}
		return nil
	})
	return defaultInterface, err
}

func (m *IpamManager) GetDefaultInterfaceAddr() (string, error) {
	var defaultInterfaceAddr string

	err := m.ipamStore.withRLock(func(st *IpamState) error {
		defaultInterfaceAddr = st.HostInterfaceAddr
		if defaultInterfaceAddr == "" {
			return fmt.Errorf("default interface address is not configured")
		}
		return nil
	})
	return defaultInterfaceAddr, err
}

func (m *IpamManager) StoreBridge(bridge string) (string, string, error) {
	var (
		bridgeSubnet string
		bridgeAddr   string
	)
	err := m.ipamStore.withLock(func(st *IpamState) error {
		for _, p := range st.Pools {
			if p.Interface == bridge {
				return fmt.Errorf("interface: %s is already created", bridge)
			}
		}

		if st.RuntimeSubnet == "" {
			return fmt.Errorf("runtime subnet is not configured")
		}

		_, runtimeNet, err := net.ParseCIDR(st.RuntimeSubnet)
		if err != nil {
			return fmt.Errorf("invalid runtime subnet: %w", err)
		}
		if runtimeNet.IP.To4() == nil {
			return fmt.Errorf("runtime subnet must be ipv4")
		}
		if ones, _ := runtimeNet.Mask.Size(); ones > 24 {
			return fmt.Errorf("runtime subnet prefix must be <= /24: %s", runtimeNet.String())
		}

		usedSubnets := make(map[string]struct{}, len(st.Pools))
		for _, p := range st.Pools {
			_, poolNet, parseErr := net.ParseCIDR(p.Subnet)
			if parseErr != nil {
				return fmt.Errorf("invalid pool subnet: %s", p.Subnet)
			}
			if poolNet.IP.To4() == nil {
				return fmt.Errorf("pool subnet must be ipv4: %s", p.Subnet)
			}
			usedSubnets[poolNet.String()] = struct{}{}
		}

		mask24 := net.CIDRMask(24, 32)
		start := runtimeNet.IP.Mask(runtimeNet.Mask).To4()
		if start == nil {
			return fmt.Errorf("runtime subnet must be ipv4")
		}

		for ip := start; runtimeNet.Contains(ip); ip = ipv4Add(ip, 256) {
			subnetNet := &net.IPNet{IP: ip, Mask: mask24}
			if !runtimeNet.Contains(broadcastIPv4(subnetNet)) {
				break
			}
			subnet := subnetNet.String()
			if _, used := usedSubnets[subnet]; used {
				continue
			}
			addr := fmt.Sprintf("%s/24", ipv4Add(ip, 254).String())
			st.Pools = append(st.Pools, Pool{
				Interface:   bridge,
				Subnet:      subnet,
				Address:     addr,
				Allocations: map[string]Allocation{},
			})
			bridgeSubnet = subnet
			bridgeAddr = addr
			return nil
		}
		return fmt.Errorf("no available /24 subnet in runtime subnet: %s", runtimeNet.String())
	})
	return bridgeSubnet, bridgeAddr, err
}

func (m *IpamManager) RemoveBridge(bridge string) error {
	return m.ipamStore.withLock(func(st *IpamState) error {
		var (
			newPools   []Pool
			removeFlag bool
		)
		for _, p := range st.Pools {
			if p.Interface == bridge {
				removeFlag = true
				continue
			}
			newPools = append(newPools, p)
		}
		if !removeFlag {
			return fmt.Errorf("bridge: %s not found", bridge)
		}
		st.Pools = newPools
		return nil
	})
}

func (m *IpamManager) GetBridgeAddr(bridgeInterface string) (string, error) {
	var bridgeAddr string

	err := m.ipamStore.withRLock(func(st *IpamState) error {
		for _, p := range st.Pools {
			if p.Interface != bridgeInterface {
				continue
			}
			bridgeAddr = p.Address
			return nil
		}
		return fmt.Errorf("interface: %s not found", bridgeInterface)
	})
	return bridgeAddr, err
}

func (m *IpamManager) GetContainerAddress(containerId string) (string, string, string, error) {
	var (
		containerAddr   string
		hostInterface   string
		bridgeInterface string
	)

	err := m.ipamStore.withRLock(func(st *IpamState) error {
		hostInterface = st.HostInterface
		for _, p := range st.Pools {
			for addr, info := range p.Allocations {
				if info.ContainerId == containerId {
					bridgeInterface = p.Interface
					containerAddr = addr
					return nil
				}
			}
		}
		return fmt.Errorf("container: %s not found", containerId)
	})
	return hostInterface, bridgeInterface, containerAddr, err
}

func (m *IpamManager) GetDnsProxyInfo() (string, string, []string, error) {
	var (
		dnsProxyInterface string
		dnsProxyAddr      string
		upstreams         []string
	)

	err := m.ipamStore.withRLock(func(st *IpamState) error {
		dnsProxyInterface = st.DnsProxy.DnsProxyInterface
		dnsProxyAddr = st.DnsProxy.DnsProxyAddr
		upstreams = st.DnsProxy.Upstreams
		if dnsProxyInterface == "" || dnsProxyAddr == "" || len(upstreams) == 0 {
			return fmt.Errorf("Dns Proxy not configured")
		}
		return nil
	})
	return dnsProxyInterface, dnsProxyAddr, upstreams, err
}

func (m *IpamManager) SetForwardInfo(containerId string, sport, dport int, protocol string) error {
	err := m.ipamStore.withLock(func(st *IpamState) error {
		for i := range st.Pools {
			p := st.Pools[i]
			if p.Allocations == nil {
				continue
			}

			for addr, info := range p.Allocations {
				if info.ContainerId != containerId {
					continue
				}

				fi := ForwardInfo{
					HostPort:      sport,
					ContainerPort: dport,
					Protocol:      protocol,
				}

				alloc := p.Allocations[addr]
				alloc.Forwards = append(alloc.Forwards, fi)
				p.Allocations[addr] = alloc

				return nil
			}
		}
		return fmt.Errorf("container: %s not found", containerId)
	})
	return err
}

func (m *IpamManager) GetForwardInfo(containerId string) ([]ForwardInfo, error) {
	var forwards []ForwardInfo
	err := m.ipamStore.withRLock(func(st *IpamState) error {
		for _, p := range st.Pools {
			if p.Allocations == nil {
				continue
			}
			for _, info := range p.Allocations {
				if info.ContainerId != containerId {
					continue
				}
				for _, f := range info.Forwards {
					forwards = append(forwards, f)
				}
			}
		}
		return nil
	})
	return forwards, err
}

func (m *IpamManager) GetPoolList() ([]Pool, error) {
	var poolList []Pool
	err := m.ipamStore.withRLock(func(st *IpamState) error {
		for _, p := range st.Pools {
			poolList = append(poolList, p)
		}
		return nil
	})
	return poolList, err
}

func (m *IpamManager) GetNetworkInfoById(containerId string) (string, Allocation, error) {
	var (
		address     string
		networkInfo Allocation
	)
	err := m.ipamStore.withRLock(func(st *IpamState) error {
		for _, p := range st.Pools {
			for addr, info := range p.Allocations {
				if info.ContainerId != containerId {
					continue
				}
				address = addr
				networkInfo = p.Allocations[addr]
				return nil
			}
		}
		return fmt.Errorf("container: %s not found", containerId)
	})
	return address, networkInfo, err
}

func (m *IpamManager) GetVethById(containerId string) (string, error) {
	var veth string
	err := m.ipamStore.withRLock(func(st *IpamState) error {
		for _, p := range st.Pools {
			for _, info := range p.Allocations {
				if info.ContainerId != containerId {
					continue
				}
				veth = info.Interface
				return nil
			}
		}
		return fmt.Errorf("contianer: %s not found", containerId)
	})
	return veth, err
}

func (m *IpamManager) GetInfoByIp(ip string) (string, string, error) {
	var (
		containerId string
		veth        string
	)
	err := m.ipamStore.withRLock(func(st *IpamState) error {
		for _, p := range st.Pools {
			for addr, info := range p.Allocations {
				if addr != ip {
					continue
				}
				containerId = info.ContainerId
				veth = info.Interface
				return nil
			}
		}
		return fmt.Errorf("ip: %s not found", ip)
	})
	return containerId, veth, err
}

func findFreeIpv4(ipnet *net.IPNet, gateway net.IP, alloc map[string]Allocation) (net.IP, error) {
	network := ipnet.IP.To4()
	if network == nil {
		return nil, fmt.Errorf("ipv4 only supported")
	}
	start := incIP(network)       // network +1
	bcast := broadcastIPv4(ipnet) // reserve: broadcast address

	// search stat
	cursor := start
	for i := 0; i < 1<<24; i++ {
		if !ipnet.Contains(cursor) {
			cursor = start
		}
		// reserve: network, gateway, broadcast
		if cursor.Equal(network) || cursor.Equal(gateway) || cursor.Equal(bcast) {
			cursor = incIP(cursor)
			continue
		}
		if _, used := alloc[cursor.String()]; !used {
			return cursor, nil
		}
		cursor = incIP(cursor)
	}

	return nil, fmt.Errorf("no free ip in subnet %s", ipnet.String())
}

func incIP(ip net.IP) net.IP {
	v := make(net.IP, len(ip))
	copy(v, ip)
	v[3]++
	for i := 3; i >= 0; i-- {
		if v[i] != 0 {
			break
		}
		if i > 0 {
			v[i-1]++
		}
	}
	return v
}

func ipv4Add(ip net.IP, add uint32) net.IP {
	v := binary.BigEndian.Uint32(ip.To4())
	v += add
	out := make(net.IP, 4)
	binary.BigEndian.PutUint32(out, v)
	return out
}

func broadcastIPv4(ipnet *net.IPNet) net.IP {
	ip := ipnet.IP.To4()
	mask := ipnet.Mask
	b := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		b[i] = ip[i] | ^mask[i]
	}
	return b
}
