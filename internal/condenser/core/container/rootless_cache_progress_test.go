package container

import (
	"os"
	"path/filepath"
	"testing"

	"raind/internal/condenser/core/image"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReportRootlessShiftedLayerCacheProgressReportsCreatingWhenCacheMissing(t *testing.T) {
	root := t.TempDir()
	layerRootfs := filepath.Join(root, "image", "layers", "library", "nginx", "latest", "rootfs")
	require.NoError(t, os.MkdirAll(layerRootfs, 0o755))

	var events []image.PullProgressEvent
	reportRootlessShiftedLayerCacheProgress(ServiceCreateModel{
		Image:    "nginx:latest",
		Rootless: true,
		Progress: func(e image.PullProgressEvent) { events = append(events, e) },
	}, layerRootfs)

	require.Len(t, events, 1)
	assert.Equal(t, "rootless-cache", events[0].Status)
	assert.Equal(t, "nginx:latest", events[0].ID)
	assert.Equal(t, "creating rootless shifted layer cache", events[0].Detail)
}

func TestReportRootlessShiftedLayerCacheProgressReportsFoundWhenCacheReady(t *testing.T) {
	root := t.TempDir()
	layerRootfs := filepath.Join(root, "image", "layers", "library", "nginx", "latest", "rootfs")
	require.NoError(t, os.MkdirAll(layerRootfs, 0o755))

	cacheRoot, err := rootlessShiftedLayerCacheRoot(layerRootfs, 100000, 100000, 65536)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(cacheRoot, "rootfs"), 0o755))
	require.NoError(t, os.WriteFile(rootlessShiftedLayerCompleteMarker(cacheRoot), []byte("complete\n"), 0o644))

	var events []image.PullProgressEvent
	reportRootlessShiftedLayerCacheProgress(ServiceCreateModel{
		Image:    "nginx:latest",
		Rootless: true,
		Progress: func(e image.PullProgressEvent) { events = append(events, e) },
	}, layerRootfs)

	require.Len(t, events, 1)
	assert.Equal(t, "rootless-cache", events[0].Status)
	assert.Equal(t, "nginx:latest", events[0].ID)
	assert.Equal(t, "rootless shifted layer cache found", events[0].Detail)
}

func TestReportRootlessShiftedLayerCacheProgressSkipsNonRootless(t *testing.T) {
	var called bool
	reportRootlessShiftedLayerCacheProgress(ServiceCreateModel{
		Image:    "nginx:latest",
		Rootless: false,
		Progress: func(image.PullProgressEvent) { called = true },
	}, "/tmp/rootfs")

	assert.False(t, called)
}
