package csm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCsmManagerStoresResolvesUpdatesAndRemovesContainer(t *testing.T) {
	manager := NewCsmManager(NewCsmStore(filepath.Join(t.TempDir(), "csm.json")))

	require.NoError(t, manager.StoreContainer(StoreContainerRequest{
		ContainerId:   "cid-1",
		State:         "created",
		Pid:           123,
		Tty:           false,
		Repository:    "library/alpine",
		Reference:     "latest",
		Command:       []string{"/bin/sh"},
		ContainerName: "web",
		LogPath:       "/tmp/init.log",
		PodId:         "pod-1",
		DropletId:     "container",
	}))

	got, err := manager.GetContainerById("cid-1")
	require.NoError(t, err)
	assert.Equal(t, "web", got.ContainerName)
	assert.Equal(t, "created", got.State)
	assert.Equal(t, "container", got.DropletId)
	assert.True(t, manager.IsNameAlreadyUsed("web"))

	list, err := manager.GetContainerList()
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "cid-1", list[0].ContainerId)

	id, err := manager.ResolveContainerId("web")
	require.NoError(t, err)
	assert.Equal(t, "cid-1", id)

	require.NoError(t, manager.UpdateContainer("cid-1", "running", 456))
	got, err = manager.GetContainerById("cid-1")
	require.NoError(t, err)
	assert.Equal(t, "running", got.State)
	assert.Equal(t, 456, got.Pid)

	require.NoError(t, manager.RemoveContainer("cid-1"))
	assert.False(t, manager.IsContainerExist("cid-1"))
}

func TestCsmStoreCorruptedJSONReturnsUsefulError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "csm.json")
	require.NoError(t, os.WriteFile(path, []byte("{broken"), 0o600))
	manager := NewCsmManager(NewCsmStore(path))

	_, err := manager.GetContainerList()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "container state json broken")
}
