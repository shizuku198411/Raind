package psm

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPsmManagerStoresPodsAndReplicaSets(t *testing.T) {
	manager := NewPsmManager(NewPsmStore(filepath.Join(t.TempDir(), "psm.json")))

	require.NoError(t, manager.StorePod(StorePodRequest{PodId: "pod-1", Name: "web", Namespace: "default", UID: "uid-1", State: PodStateCreated, Labels: map[string]string{"app": "web"}, Rootless: true}))
	pod, err := manager.GetPodById("pod-1")
	require.NoError(t, err)
	assert.Equal(t, "web", pod.Name)
	assert.True(t, pod.Rootless)

	require.NoError(t, manager.UpdatePodOwner("pod-1", OwnerKindReplicaSet, "rs-1"))
	pod, err = manager.GetPodById("pod-1")
	require.NoError(t, err)
	assert.Equal(t, OwnerKindReplicaSet, pod.OwnerKind)
	assert.Equal(t, "rs-1", pod.OwnerId)

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

	next := time.Now().Add(time.Minute).Truncate(time.Second)
	require.NoError(t, manager.UpdateReplicaSetReconcileStatus("rs-1", 2, "create failed", next))
	rs, err = manager.GetReplicaSet("rs-1")
	require.NoError(t, err)
	assert.Equal(t, 2, rs.ReconcileAttempt)
	assert.Equal(t, "create failed", rs.LastReconcileError)
	assert.True(t, next.Equal(rs.NextReconcileAt), "expected %s, got %s", next, rs.NextReconcileAt)

	require.NoError(t, manager.ClearReplicaSetReconcileStatus("rs-1"))
	rs, err = manager.GetReplicaSet("rs-1")
	require.NoError(t, err)
	assert.Zero(t, rs.ReconcileAttempt)
	assert.Empty(t, rs.LastReconcileError)
	assert.True(t, rs.NextReconcileAt.IsZero())

	require.NoError(t, manager.RemoveReplicaSet("rs-1"))
	require.NoError(t, manager.RemovePod("pod-1"))
}
