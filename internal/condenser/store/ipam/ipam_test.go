package ipam

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIpamManagerAllocatesReleasesAndManagesBridge(t *testing.T) {
	store := NewIpamStore(filepath.Join(t.TempDir(), "ipam.json"))
	require.NoError(t, store.atomicSave(&IpamState{
		Version:       "0.1.0",
		RuntimeSubnet: "10.166.0.0/16",
		Pools: []Pool{{
			Interface:   "raind0",
			Subnet:      "10.166.0.0/24",
			Address:     "10.166.0.254/24",
			Allocations: map[string]Allocation{},
		}},
	}))
	manager := NewIpamManager(store)

	addr, err := manager.Allocate("cid-1", "raind0")
	require.NoError(t, err)
	assert.NotEmpty(t, addr)

	host, bridge, gotAddr, err := manager.GetContainerAddress("cid-1")
	require.NoError(t, err)
	assert.Equal(t, "raind0", bridge)
	assert.Equal(t, addr, gotAddr)
	assert.Empty(t, host)

	networks, err := manager.GetNetworkList()
	require.NoError(t, err)
	require.Len(t, networks, 1)
	assert.Equal(t, "raind0", networks[0].Interface)
	assert.Equal(t, 1, networks[0].NumContainers)

	require.NoError(t, manager.Release("cid-1"))
	_, _, _, err = manager.GetContainerAddress("cid-1")
	require.Error(t, err)

	subnet, bridgeAddr, err := manager.StoreBridge("raind1")
	require.NoError(t, err)
	assert.Equal(t, "10.166.1.0/24", subnet)
	assert.Equal(t, "10.166.1.254/24", bridgeAddr)

	_, _, err = manager.StoreBridge("raind1")
	require.Error(t, err)
	require.NoError(t, manager.RemoveBridge("raind1"))
	require.Error(t, manager.RemoveBridge("raind1"))
}

func TestIpamStoreAtomicSaveWritesValidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ipam.json")
	store := NewIpamStore(path)

	require.NoError(t, store.atomicSave(&IpamState{Version: "test", Pools: []Pool{}}))

	var got IpamState
	require.NoError(t, json.Unmarshal(mustReadFile(t, path), &got))
	assert.Equal(t, "test", got.Version)
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return b
}
