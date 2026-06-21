package dns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"raind/internal/condenser/store/bsm"
	"raind/internal/condenser/store/csm"
	"raind/internal/condenser/store/ipam"
	"raind/internal/condenser/store/ssm"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRaindLocalName(t *testing.T) {
	containerName, networkName, ok := parseRaindLocalName("db.raind0.raind.")

	require.True(t, ok)
	assert.Equal(t, "db", containerName)
	assert.Equal(t, "raind0", networkName)
}

func TestParseRaindLocalNameRejectsInvalidLabels(t *testing.T) {
	for _, name := range []string{
		"db.my_network.raind.",
		"db.-raind0.raind.",
		"db.raind0-.raind.",
		"db.raind.",
		"db.raind0.example.",
	} {
		t.Run(name, func(t *testing.T) {
			_, _, ok := parseRaindLocalName(name)
			assert.False(t, ok)
		})
	}
}

func TestParseClusterServiceName(t *testing.T) {
	serviceName, namespace, ok := parseClusterServiceName("db.default.svc.cluster.local.")

	require.True(t, ok)
	assert.Equal(t, "db", serviceName)
	assert.Equal(t, "default", namespace)

	serviceName, namespace, ok = parseClusterServiceName("api.demo.svc.")
	require.True(t, ok)
	assert.Equal(t, "api", serviceName)
	assert.Equal(t, "demo", namespace)
}

func TestParseClusterServiceNameRejectsInvalidLabels(t *testing.T) {
	for _, name := range []string{
		"db.default.cluster.local.",
		"db.svc.cluster.local.",
		"db.default.svc.example.local.",
		"db.my_namespace.svc.cluster.local.",
		"db.-default.svc.cluster.local.",
		"db.default-.svc.cluster.local.",
	} {
		t.Run(name, func(t *testing.T) {
			_, _, ok := parseClusterServiceName(name)
			assert.False(t, ok)
		})
	}
}

func TestResolveClusterServiceReturnsClusterIPARecord(t *testing.T) {
	proxy := newTestDnsProxy(t)
	req := new(dns.Msg)
	req.SetQuestion("db.default.svc.cluster.local.", dns.TypeA)

	resp, ok := proxy.resolveRaindLocal(req)

	require.True(t, ok)
	require.NotNil(t, resp)
	assert.Equal(t, dns.RcodeSuccess, resp.Rcode)
	require.Len(t, resp.Answer, 1)
	a, ok := resp.Answer[0].(*dns.A)
	require.True(t, ok)
	assert.Equal(t, "10.166.255.10", a.A.String())
	assert.Equal(t, uint32(clusterServiceDNSTTL), a.Hdr.Ttl)
}

func TestResolveClusterServiceReturnsClusterIPForShortSvcDomain(t *testing.T) {
	proxy := newTestDnsProxy(t)
	req := new(dns.Msg)
	req.SetQuestion("db.default.svc.", dns.TypeA)

	resp, ok := proxy.resolveRaindLocal(req)

	require.True(t, ok)
	require.NotNil(t, resp)
	assert.Equal(t, dns.RcodeSuccess, resp.Rcode)
	require.Len(t, resp.Answer, 1)
	a, ok := resp.Answer[0].(*dns.A)
	require.True(t, ok)
	assert.Equal(t, "10.166.255.10", a.A.String())
}

func TestResolveClusterServiceReturnsNXDOMAINForMissingService(t *testing.T) {
	proxy := newTestDnsProxy(t)
	req := new(dns.Msg)
	req.SetQuestion("missing.default.svc.cluster.local.", dns.TypeA)

	resp, ok := proxy.resolveRaindLocal(req)

	require.True(t, ok)
	require.NotNil(t, resp)
	assert.Equal(t, dns.RcodeNameError, resp.Rcode)
	assert.Empty(t, resp.Answer)
}

func TestResolveClusterServiceReturnsNXDOMAINForNonClusterIPService(t *testing.T) {
	proxy := newTestDnsProxy(t)
	req := new(dns.Msg)
	req.SetQuestion("node.default.svc.cluster.local.", dns.TypeA)

	resp, ok := proxy.resolveRaindLocal(req)

	require.True(t, ok)
	require.NotNil(t, resp)
	assert.Equal(t, dns.RcodeNameError, resp.Rcode)
	assert.Empty(t, resp.Answer)
}

func TestResolveClusterServiceDoesNotForwardUnsupportedQueryTypes(t *testing.T) {
	proxy := newTestDnsProxy(t)
	req := new(dns.Msg)
	req.SetQuestion("db.default.svc.cluster.local.", dns.TypeAAAA)

	resp, ok := proxy.resolveRaindLocal(req)

	require.True(t, ok)
	require.NotNil(t, resp)
	assert.Equal(t, dns.RcodeNameError, resp.Rcode)
}

func TestResolveRaindLocalReturnsARecordFromIPAMAndCSM(t *testing.T) {
	proxy := newTestDnsProxy(t)
	req := new(dns.Msg)
	req.SetQuestion("db.raind0.raind.", dns.TypeA)

	resp, ok := proxy.resolveRaindLocal(req)

	require.True(t, ok)
	require.NotNil(t, resp)
	assert.Equal(t, dns.RcodeSuccess, resp.Rcode)
	require.Len(t, resp.Answer, 1)
	a, ok := resp.Answer[0].(*dns.A)
	require.True(t, ok)
	assert.Equal(t, "10.166.0.2", a.A.String())
	assert.Equal(t, uint32(raindLocalTTL), a.Hdr.Ttl)
}

func TestResolveRaindLocalReturnsARecordForBottleServiceAlias(t *testing.T) {
	proxy := newTestDnsProxy(t)
	req := new(dns.Msg)
	req.SetQuestion("api.raind0.raind.", dns.TypeA)

	resp, ok := proxy.resolveRaindLocal(req)

	require.True(t, ok)
	require.NotNil(t, resp)
	assert.Equal(t, dns.RcodeSuccess, resp.Rcode)
	require.Len(t, resp.Answer, 1)
	a, ok := resp.Answer[0].(*dns.A)
	require.True(t, ok)
	assert.Equal(t, "10.166.0.3", a.A.String())
}

func TestResolveRaindLocalReturnsNXDOMAINForUnknownContainer(t *testing.T) {
	proxy := newTestDnsProxy(t)
	req := new(dns.Msg)
	req.SetQuestion("missing.raind0.raind.", dns.TypeA)

	resp, ok := proxy.resolveRaindLocal(req)

	require.True(t, ok)
	require.NotNil(t, resp)
	assert.Equal(t, dns.RcodeNameError, resp.Rcode)
	assert.Empty(t, resp.Answer)
}

func TestResolveRaindLocalIgnoresNonRaindZone(t *testing.T) {
	proxy := newTestDnsProxy(t)
	req := new(dns.Msg)
	req.SetQuestion("example.com.", dns.TypeA)

	resp, ok := proxy.resolveRaindLocal(req)

	assert.False(t, ok)
	assert.Nil(t, resp)
}

func TestResolveRaindLocalDoesNotForwardUnsupportedRaindQueryTypes(t *testing.T) {
	proxy := newTestDnsProxy(t)
	req := new(dns.Msg)
	req.SetQuestion("db.raind0.raind.", dns.TypeAAAA)

	resp, ok := proxy.resolveRaindLocal(req)

	require.True(t, ok)
	require.NotNil(t, resp)
	assert.Equal(t, dns.RcodeNameError, resp.Rcode)
}

func newTestDnsProxy(t *testing.T) *DnsProxy {
	t.Helper()
	dir := t.TempDir()

	ipamPath := filepath.Join(dir, "ipam.json")
	csmPath := filepath.Join(dir, "csm.json")
	bsmPath := filepath.Join(dir, "bsm.json")
	ssmPath := filepath.Join(dir, "ssm.json")

	writeJSON(t, ipamPath, ipam.IpamState{
		Version:       "test",
		RuntimeSubnet: "10.166.0.0/16",
		Pools: []ipam.Pool{{
			Interface: "raind0",
			Subnet:    "10.166.0.0/24",
			Address:   "10.166.0.254/24",
			Allocations: map[string]ipam.Allocation{
				"10.166.0.2": {
					ContainerId: "cid-db",
					Interface:   "rd_cid-db",
					AssignedAt:  time.Now(),
				},
				"10.166.0.3": {
					ContainerId: "cid-api",
					Interface:   "rd_cid-api",
					AssignedAt:  time.Now(),
				},
			},
		}},
	})
	writeJSON(t, csmPath, csm.ContainerState{
		Version: "test",
		Containers: map[string]csm.ContainerInfo{
			"cid-db": {
				ContainerId:   "cid-db",
				ContainerName: "db",
				State:         "running",
			},
			"cid-api": {
				ContainerId:   "cid-api",
				ContainerName: "test-bottle-api",
				State:         "running",
			},
		},
	})
	writeJSON(t, ssmPath, ssm.ServiceState{
		Version: "test",
		Services: map[string]ssm.ServiceInfo{
			"svc-db": {
				ServiceId: "svc-db",
				Name:      "db",
				Namespace: "default",
				Type:      ssm.ServiceTypeClusterIP,
				ClusterIP: "10.166.255.10",
			},
			"svc-default-type": {
				ServiceId: "svc-default-type",
				Name:      "api",
				Namespace: "default",
				ClusterIP: "10.166.255.11",
			},
			"svc-node": {
				ServiceId: "svc-node",
				Name:      "node",
				Namespace: "default",
				Type:      ssm.ServiceTypeNodePort,
			},
		},
	})

	writeJSON(t, bsmPath, bsm.BottleState{
		Version: "test",
		Bottles: map[string]bsm.BottleInfo{
			"bot-1": {
				BottleId:   "bot-1",
				BottleName: "test-bottle",
				Network:    "raind0",
				Services: map[string]bsm.ServiceSpec{
					"api": {Image: "alpine:latest"},
				},
				Containers: map[string]string{
					"api": "cid-api",
				},
			},
		},
	})

	return &DnsProxy{
		csmHandler:  csm.NewCsmManager(csm.NewCsmStore(csmPath)),
		ipamHandler: ipam.NewIpamManager(ipam.NewIpamStore(ipamPath)),
		bsmHandler:  bsm.NewBsmManager(bsm.NewBsmStore(bsmPath)),
		ssmHandler:  ssm.NewSsmManager(ssm.NewSsmStore(ssmPath)),
	}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, append(b, '\n'), 0o600))
}
