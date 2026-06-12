package bsm

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBsmManagerStoresResolvesUpdatesAndRemovesBottle(t *testing.T) {
	manager := NewBsmManager(NewBsmStore(filepath.Join(t.TempDir(), "bsm.json")))
	services := map[string]ServiceSpec{
		"web": {Image: "alpine:latest", DependsOn: []string{"db"}},
		"db":  {Image: "postgres:latest"},
	}

	require.NoError(t, manager.StoreBottle("bot-1", "app", services, []string{"db", "web"}, nil))
	assert.True(t, manager.IsNameAlreadyUsed("app"))

	id, err := manager.ResolveBottleId("app")
	require.NoError(t, err)
	assert.Equal(t, "bot-1", id)

	require.NoError(t, manager.UpdateBottleContainer("bot-1", "web", "cid-1"))
	got, err := manager.GetBottleById("bot-1")
	require.NoError(t, err)
	assert.Equal(t, "cid-1", got.Containers["web"])

	require.NoError(t, manager.RemoveBottle("bot-1"))
	_, err = manager.GetBottleById("bot-1")
	require.Error(t, err)
}
