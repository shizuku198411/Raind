package nsm

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNsmStoreInitializesDefaultNamespace(t *testing.T) {
	manager := NewNsmManager(NewNsmStore(filepath.Join(t.TempDir(), "nsm.json")))

	require.NoError(t, manager.EnsureDefaultNamespace())
	info, err := manager.GetNamespace(DefaultNamespace)
	require.NoError(t, err)

	assert.Equal(t, DefaultNamespace, info.Name)
	assert.Equal(t, DefaultNamespaceNetwork, info.Network)
	assert.False(t, info.NetworkAuto)
}

func TestNsmManagerRejectsDuplicateNamespace(t *testing.T) {
	manager := NewNsmManager(NewNsmStore(filepath.Join(t.TempDir(), "nsm.json")))

	require.NoError(t, manager.StoreNamespace(NamespaceInfo{Name: "dev", Network: "rns-dev", NetworkAuto: true}))
	err := manager.StoreNamespace(NamespaceInfo{Name: "dev", Network: "rns-dev2", NetworkAuto: true})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace already exists")
}

func TestNsmManagerDoesNotRemoveDefaultNamespace(t *testing.T) {
	manager := NewNsmManager(NewNsmStore(filepath.Join(t.TempDir(), "nsm.json")))

	err := manager.RemoveNamespace(DefaultNamespace)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "default namespace cannot be removed")
}
