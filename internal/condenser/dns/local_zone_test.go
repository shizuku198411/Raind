package dns

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"raind/internal/condenser/store/csm"
	"raind/internal/condenser/store/ipam"

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
		},
	})

	return &DnsProxy{
		csmHandler:  csm.NewCsmManager(csm.NewCsmStore(csmPath)),
		ipamHandler: ipam.NewIpamManager(ipam.NewIpamStore(ipamPath)),
	}
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, append(b, '\n'), 0o600))
}
