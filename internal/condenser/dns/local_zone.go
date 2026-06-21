package dns

import (
	"fmt"
	"net"
	"strings"

	"raind/internal/condenser/store/ipam"
	"raind/internal/condenser/store/ssm"

	"github.com/miekg/dns"
)

const (
	raindLocalZone       = "raind."
	clusterLocalZone     = "cluster.local."
	clusterServiceZone   = "svc.cluster.local."
	shortServiceZone     = "svc."
	raindLocalTTL        = 5
	clusterServiceDNSTTL = 5
)

func (f *DnsProxy) resolveRaindLocal(req *dns.Msg) (*dns.Msg, bool) {
	if msg, ok := f.resolveClusterService(req); ok {
		return msg, true
	}
	if req == nil || len(req.Question) != 1 || req.Opcode != dns.OpcodeQuery || req.Response {
		return nil, false
	}

	q := req.Question[0]
	if q.Qclass != dns.ClassINET {
		return nil, false
	}
	if q.Qtype != dns.TypeA && q.Qtype != dns.TypeANY {
		if isRaindLocalName(q.Name) {
			return newRaindLocalReply(req, dns.RcodeNameError), true
		}
		return nil, false
	}

	containerName, networkName, ok := parseRaindLocalName(q.Name)
	if !ok {
		if isRaindLocalName(q.Name) {
			return newRaindLocalReply(req, dns.RcodeNameError), true
		}
		return nil, false
	}

	addr, err := f.lookupContainerAddress(networkName, containerName)
	if err != nil {
		return newRaindLocalReply(req, dns.RcodeNameError), true
	}

	ip := net.ParseIP(addr).To4()
	if ip == nil {
		return newRaindLocalReply(req, dns.RcodeNameError), true
	}

	resp := newRaindLocalReply(req, dns.RcodeSuccess)
	resp.Authoritative = true
	resp.Answer = append(resp.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(strings.ToLower(q.Name)),
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    raindLocalTTL,
		},
		A: ip,
	})
	return resp, true
}

func (f *DnsProxy) resolveClusterService(req *dns.Msg) (*dns.Msg, bool) {
	if req == nil || len(req.Question) != 1 || req.Opcode != dns.OpcodeQuery || req.Response {
		return nil, false
	}

	q := req.Question[0]
	if q.Qclass != dns.ClassINET {
		return nil, false
	}

	serviceName, namespace, ok := parseClusterServiceName(q.Name)
	if !ok {
		if isClusterLocalName(q.Name) || isShortServiceName(q.Name) {
			return newRaindLocalReply(req, dns.RcodeNameError), true
		}
		return nil, false
	}

	if q.Qtype != dns.TypeA && q.Qtype != dns.TypeANY {
		return newRaindLocalReply(req, dns.RcodeNameError), true
	}

	addr, err := f.lookupClusterIPService(namespace, serviceName)
	if err != nil {
		return newRaindLocalReply(req, dns.RcodeNameError), true
	}

	ip := net.ParseIP(addr).To4()
	if ip == nil {
		return newRaindLocalReply(req, dns.RcodeNameError), true
	}

	resp := newRaindLocalReply(req, dns.RcodeSuccess)
	resp.Authoritative = true
	resp.Answer = append(resp.Answer, &dns.A{
		Hdr: dns.RR_Header{
			Name:   dns.Fqdn(strings.ToLower(q.Name)),
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    clusterServiceDNSTTL,
		},
		A: ip,
	})
	return resp, true
}

func newRaindLocalReply(req *dns.Msg, rcode int) *dns.Msg {
	m := new(dns.Msg)
	m.SetReply(req)
	m.RecursionAvailable = true
	m.Authoritative = true
	m.Rcode = rcode
	return m
}

func isRaindLocalName(name string) bool {
	return strings.HasSuffix(strings.ToLower(dns.Fqdn(name)), raindLocalZone)
}

func isClusterLocalName(name string) bool {
	return strings.HasSuffix(strings.ToLower(dns.Fqdn(name)), clusterLocalZone)
}

func isShortServiceName(name string) bool {
	return strings.HasSuffix(strings.ToLower(dns.Fqdn(name)), shortServiceZone)
}

func parseRaindLocalName(name string) (containerName string, networkName string, ok bool) {
	fqdn := strings.TrimSuffix(strings.ToLower(dns.Fqdn(name)), ".")
	zone := strings.TrimSuffix(raindLocalZone, ".")
	if !strings.HasSuffix(fqdn, "."+zone) {
		return "", "", false
	}

	left := strings.TrimSuffix(fqdn, "."+zone)
	parts := strings.Split(left, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	if !isDnsLabel(parts[0]) || !isDnsLabel(parts[1]) {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func parseClusterServiceName(name string) (serviceName string, namespace string, ok bool) {
	fqdn := strings.TrimSuffix(strings.ToLower(dns.Fqdn(name)), ".")

	if strings.HasSuffix(fqdn, "."+strings.TrimSuffix(clusterServiceZone, ".")) {
		left := strings.TrimSuffix(fqdn, "."+strings.TrimSuffix(clusterServiceZone, "."))
		parts := strings.Split(left, ".")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", false
		}
		if !isDnsLabel(parts[0]) || !isDnsLabel(parts[1]) {
			return "", "", false
		}
		return parts[0], parts[1], true
	}

	if strings.HasSuffix(fqdn, "."+strings.TrimSuffix(shortServiceZone, ".")) {
		left := strings.TrimSuffix(fqdn, "."+strings.TrimSuffix(shortServiceZone, "."))
		parts := strings.Split(left, ".")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return "", "", false
		}
		if !isDnsLabel(parts[0]) || !isDnsLabel(parts[1]) {
			return "", "", false
		}
		return parts[0], parts[1], true
	}

	return "", "", false
}

func isDnsLabel(s string) bool {
	if len(s) == 0 || len(s) > 63 {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
		case r == '-':
			if i == 0 || i == len(s)-1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func (f *DnsProxy) lookupClusterIPService(namespace string, serviceName string) (string, error) {
	if f == nil || f.ssmHandler == nil {
		return "", fmt.Errorf("dns service resolver is not configured")
	}

	services, err := f.ssmHandler.GetServiceList()
	if err != nil {
		return "", err
	}
	for _, svc := range services {
		if svc.Name != serviceName || svc.Namespace != namespace {
			continue
		}
		if serviceType(svc) != ssm.ServiceTypeClusterIP || strings.TrimSpace(svc.ClusterIP) == "" {
			return "", fmt.Errorf("service %s/%s has no ClusterIP", namespace, serviceName)
		}
		return strings.TrimSpace(svc.ClusterIP), nil
	}
	return "", fmt.Errorf("service %s/%s not found", namespace, serviceName)
}

func serviceType(svc ssm.ServiceInfo) string {
	if strings.TrimSpace(svc.Type) == "" {
		return ssm.ServiceTypeClusterIP
	}
	return svc.Type
}

func (f *DnsProxy) lookupContainerAddress(networkName string, containerName string) (string, error) {
	if f == nil || f.ipamHandler == nil || f.csmHandler == nil {
		return "", fmt.Errorf("dns local resolver is not configured")
	}

	pools, err := f.ipamHandler.GetPoolList()
	if err != nil {
		return "", err
	}

	for _, pool := range pools {
		if pool.Interface != networkName {
			continue
		}

		if addr, ok := f.lookupDirectContainerName(pool.Allocations, containerName); ok {
			return addr, nil
		}
		if addr, ok := f.lookupBottleServiceAlias(networkName, pool.Allocations, containerName); ok {
			return addr, nil
		}

		return "", fmt.Errorf("container or service %s not found in network %s", containerName, networkName)
	}
	return "", fmt.Errorf("network %s not found", networkName)
}

func (f *DnsProxy) lookupDirectContainerName(allocations map[string]ipam.Allocation, containerName string) (string, bool) {
	for addr, alloc := range allocations {
		name, err := f.csmHandler.GetContainerNameById(alloc.ContainerId)
		if err != nil {
			continue
		}
		if name == containerName {
			return addr, true
		}
	}
	return "", false
}

func (f *DnsProxy) lookupBottleServiceAlias(networkName string, allocations map[string]ipam.Allocation, serviceName string) (string, bool) {
	if f.bsmHandler == nil {
		return "", false
	}

	bottles, err := f.bsmHandler.GetBottleList()
	if err != nil {
		return "", false
	}
	for _, bottle := range bottles {
		if bottle.Network != networkName {
			continue
		}
		containerId := bottle.Containers[serviceName]
		if containerId == "" {
			continue
		}
		for addr, alloc := range allocations {
			if alloc.ContainerId == containerId {
				return addr, true
			}
		}
	}
	return "", false
}
