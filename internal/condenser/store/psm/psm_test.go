package psm

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPsmManagerStoresPodsAndReplicaSets(t *testing.T) {
	manager := NewPsmManager(NewPsmStore(filepath.Join(t.TempDir(), "psm.json")))

	require.NoError(t, manager.StorePod(StorePodRequest{PodId: "pod-1", Name: "web", Namespace: "default", UID: "uid-1", State: "created", Labels: map[string]string{"app": "web"}, Rootless: true}))
	pod, err := manager.GetPodById("pod-1")
	require.NoError(t, err)
	assert.Equal(t, "web", pod.Name)
	assert.True(t, pod.Rootless)

	require.NoError(t, manager.StorePodTemplate("tpl-1", PodTemplateSpec{Name: "web", Namespace: "default", Rootless: true}))
	require.NoError(t, manager.StoreReplicaSet("rs-1", ReplicaSetSpec{Name: "web", Namespace: "default", Replicas: 2, TemplateId: "tpl-1"}))
	rs, err := manager.GetReplicaSet("rs-1")
	require.NoError(t, err)
	assert.Equal(t, 2, rs.Spec.Replicas)
	tpl, err := manager.GetPodTemplate("tpl-1")
	require.NoError(t, err)
	assert.True(t, tpl.Spec.Rootless)

	require.NoError(t, manager.UpdateReplicaSetReplicas("rs-1", 3))
	rs, err = manager.GetReplicaSet("rs-1")
	require.NoError(t, err)
	assert.Equal(t, 3, rs.Spec.Replicas)

	require.NoError(t, manager.RemoveReplicaSet("rs-1"))
	require.NoError(t, manager.RemovePod("pod-1"))
}
