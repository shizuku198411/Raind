package ssm

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSsmManagerStoresListsGetsAndRemovesService(t *testing.T) {
	manager := NewSsmManager(NewSsmStore(filepath.Join(t.TempDir(), "ssm.json")))
	spec := ServiceInfo{
		Name:      "web",
		Namespace: "default",
		Selector:  map[string]string{"app": "web"},
		Ports:     []ServicePort{{Port: 80, TargetPort: 8080, Protocol: "TCP"}},
	}

	require.NoError(t, manager.StoreService("svc-1", spec))
	assert.True(t, manager.IsNameAlreadyUsed("web", "default"))

	got, err := manager.GetServiceById("svc-1")
	require.NoError(t, err)
	assert.Equal(t, "web", got.Name)
	assert.Equal(t, "svc-1", got.ServiceId)

	list, err := manager.GetServiceList()
	require.NoError(t, err)
	require.Len(t, list, 1)

	require.NoError(t, manager.RemoveService("svc-1"))
	assert.False(t, manager.IsNameAlreadyUsed("web", "default"))
}
