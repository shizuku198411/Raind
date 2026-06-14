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
		Type:      ServiceTypeNodePort,
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

func TestSsmManagerDefaultsServiceTypeToClusterIP(t *testing.T) {
	manager := NewSsmManager(NewSsmStore(filepath.Join(t.TempDir(), "ssm.json")))

	require.NoError(t, manager.StoreService("svc-1", ServiceInfo{
		Name:      "web",
		Namespace: "default",
		Selector:  map[string]string{"app": "web"},
		Ports:     []ServicePort{{Port: 80, TargetPort: 8080, Protocol: "TCP"}},
	}))

	web, err := manager.GetServiceById("svc-1")
	require.NoError(t, err)
	assert.Equal(t, ServiceTypeClusterIP, web.Type)
	assert.Equal(t, "10.166.255.1", web.ClusterIP)
}

func TestSsmManagerAllocatesClusterIP(t *testing.T) {
	manager := NewSsmManager(NewSsmStore(filepath.Join(t.TempDir(), "ssm.json")))

	require.NoError(t, manager.StoreService("svc-1", ServiceInfo{
		Name:      "web",
		Namespace: "default",
		Type:      ServiceTypeClusterIP,
		Selector:  map[string]string{"app": "web"},
		Ports:     []ServicePort{{Port: 80, TargetPort: 8080, Protocol: "TCP"}},
	}))
	require.NoError(t, manager.StoreService("svc-2", ServiceInfo{
		Name:      "api",
		Namespace: "default",
		Type:      ServiceTypeClusterIP,
		ClusterIP: "10.166.255.10",
		Selector:  map[string]string{"app": "api"},
		Ports:     []ServicePort{{Port: 80, TargetPort: 8080, Protocol: "TCP"}},
	}))

	web, err := manager.GetServiceById("svc-1")
	require.NoError(t, err)
	assert.Equal(t, ServiceTypeClusterIP, web.Type)
	assert.Equal(t, "10.166.255.1", web.ClusterIP)

	api, err := manager.GetServiceById("svc-2")
	require.NoError(t, err)
	assert.Equal(t, "10.166.255.10", api.ClusterIP)
}

func TestSsmManagerRejectsDuplicateClusterIP(t *testing.T) {
	manager := NewSsmManager(NewSsmStore(filepath.Join(t.TempDir(), "ssm.json")))

	require.NoError(t, manager.StoreService("svc-1", ServiceInfo{
		Name:      "web",
		Namespace: "default",
		Type:      ServiceTypeClusterIP,
		ClusterIP: "10.166.255.10",
	}))

	err := manager.StoreService("svc-2", ServiceInfo{
		Name:      "api",
		Namespace: "default",
		Type:      ServiceTypeClusterIP,
		ClusterIP: "10.166.255.10",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already allocated")
}
