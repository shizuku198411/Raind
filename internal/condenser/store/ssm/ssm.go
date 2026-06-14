package ssm

import (
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	ServiceTypeNodePort  = "NodePort"
	ServiceTypeClusterIP = "ClusterIP"
	DefaultServiceCIDR   = "10.166.255.0/24"
)

func NewSsmManager(ssmStore *SsmStore) *SsmManager {
	return &SsmManager{
		ssmStore: ssmStore,
	}
}

type SsmManager struct {
	ssmStore *SsmStore
}

func (m *SsmManager) StoreService(serviceId string, spec ServiceInfo) error {
	return m.ssmStore.withLock(func(st *ServiceState) error {
		spec.ServiceId = serviceId
		spec.CreatedAt = time.Now()
		spec.Type = NormalizeServiceType(spec.Type)
		if spec.Type != ServiceTypeNodePort && spec.Type != ServiceTypeClusterIP {
			return fmt.Errorf("unsupported service type: %s", spec.Type)
		}
		if spec.Type == ServiceTypeClusterIP {
			clusterIP, err := allocateClusterIP(st, spec.ClusterIP)
			if err != nil {
				return err
			}
			spec.ClusterIP = clusterIP
		}
		st.Services[serviceId] = spec
		return nil
	})
}

func NormalizeServiceType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "", "clusterip":
		return ServiceTypeClusterIP
	case "nodeport":
		return ServiceTypeNodePort
	default:
		return t
	}
}

func allocateClusterIP(st *ServiceState, requested string) (string, error) {
	_, ipnet, err := net.ParseCIDR(DefaultServiceCIDR)
	if err != nil {
		return "", err
	}
	used := map[string]struct{}{}
	for _, svc := range st.Services {
		if svc.ClusterIP != "" {
			used[svc.ClusterIP] = struct{}{}
		}
	}

	if requested != "" {
		ip := net.ParseIP(requested).To4()
		if ip == nil {
			return "", fmt.Errorf("invalid clusterIP: %s", requested)
		}
		if !ipnet.Contains(ip) {
			return "", fmt.Errorf("clusterIP %s is outside service CIDR %s", requested, DefaultServiceCIDR)
		}
		if isReservedClusterIP(ipnet, ip) {
			return "", fmt.Errorf("clusterIP %s is reserved", requested)
		}
		if _, exists := used[ip.String()]; exists {
			return "", fmt.Errorf("clusterIP %s is already allocated", requested)
		}
		return ip.String(), nil
	}

	start := incIPv4(ipnet.IP.To4())
	for ip := start; ipnet.Contains(ip); ip = incIPv4(ip) {
		if isReservedClusterIP(ipnet, ip) {
			continue
		}
		if _, exists := used[ip.String()]; exists {
			continue
		}
		return ip.String(), nil
	}
	return "", fmt.Errorf("no available clusterIP in service CIDR %s", DefaultServiceCIDR)
}

func isReservedClusterIP(ipnet *net.IPNet, ip net.IP) bool {
	network := ipnet.IP.To4()
	broadcast := broadcastIPv4(ipnet)
	return ip.Equal(network) || ip.Equal(broadcast)
}

func broadcastIPv4(ipnet *net.IPNet) net.IP {
	ip := ipnet.IP.To4()
	mask := ipnet.Mask
	out := make(net.IP, 4)
	for i := 0; i < 4; i++ {
		out[i] = ip[i] | ^mask[i]
	}
	return out
}

func incIPv4(ip net.IP) net.IP {
	out := make(net.IP, len(ip))
	copy(out, ip)
	for i := len(out) - 1; i >= 0; i-- {
		out[i]++
		if out[i] != 0 {
			break
		}
	}
	return out
}

func (m *SsmManager) GetServiceList() ([]ServiceInfo, error) {
	var list []ServiceInfo
	err := m.ssmStore.withRLock(func(st *ServiceState) error {
		for _, s := range st.Services {
			s.Type = NormalizeServiceType(s.Type)
			list = append(list, s)
		}
		return nil
	})
	return list, err
}

func (m *SsmManager) GetServiceById(serviceId string) (ServiceInfo, error) {
	var info ServiceInfo
	err := m.ssmStore.withRLock(func(st *ServiceState) error {
		s, ok := st.Services[serviceId]
		if !ok {
			return fmt.Errorf("serviceId=%s not found", serviceId)
		}
		s.Type = NormalizeServiceType(s.Type)
		info = s
		return nil
	})
	return info, err
}

func (m *SsmManager) RemoveService(serviceId string) error {
	return m.ssmStore.withLock(func(st *ServiceState) error {
		if _, ok := st.Services[serviceId]; !ok {
			return fmt.Errorf("serviceId=%s not found", serviceId)
		}
		delete(st.Services, serviceId)
		return nil
	})
}

func (m *SsmManager) IsNameAlreadyUsed(name, namespace string) bool {
	var used bool
	_ = m.ssmStore.withRLock(func(st *ServiceState) error {
		for _, s := range st.Services {
			if s.Name == name && s.Namespace == namespace {
				used = true
				return nil
			}
		}
		return nil
	})
	return used
}
