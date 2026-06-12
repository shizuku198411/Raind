package ilm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIlmManagerStoresListsGetsAndRemovesImage(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(configPath, []byte("{}"), 0o600))
	manager := NewIlmManager(NewIlmStore(filepath.Join(dir, "ilm.json")))

	require.NoError(t, manager.StoreImage("library/alpine", "latest", "/bundle", configPath, "/rootfs"))

	assert.True(t, manager.IsImageExist("library/alpine", "latest"))
	list, err := manager.GetImageList()
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "library/alpine", list[0].Repository)
	assert.Equal(t, "latest", list[0].Reference)

	bundle, err := manager.GetBundlePath("library/alpine", "latest")
	require.NoError(t, err)
	assert.Equal(t, "/bundle", bundle)

	require.NoError(t, manager.RemoveImage("library/alpine", "latest"))
	assert.False(t, manager.IsImageExist("library/alpine", "latest"))
}

func TestIlmStoreCorruptedJSONReturnsUsefulError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ilm.json")
	require.NoError(t, os.WriteFile(path, []byte("{broken"), 0o600))
	manager := NewIlmManager(NewIlmStore(path))

	_, err := manager.GetImageList()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "image layer state json broken")
}
