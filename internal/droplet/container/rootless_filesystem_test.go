package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareRootlessShiftedImageLayersCreatesReusableCache(t *testing.T) {
	// == setup ==
	root := t.TempDir()
	t.Setenv("RAIND_ROOTLESS_UID_BASE", "200000")
	t.Setenv("RAIND_ROOTLESS_GID_BASE", "300000")
	t.Setenv("RAIND_ROOTLESS_ID_MAP_SIZE", "65536")

	layerRootfs := filepath.Join(root, "image", "layers", "library", "nginx", "latest", "rootfs")
	require.NoError(t, os.MkdirAll(filepath.Join(layerRootfs, "var", "log"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(layerRootfs, "var", "log", "app.log"), []byte("hello"), 0o644))
	originalUID := os.Getuid()
	originalGID := os.Getgid()
	chownCalls := map[string][2]int{}
	originalLchown := rootlessLchown
	rootlessLchown = func(path string, uid int, gid int) error {
		chownCalls[filepath.Clean(path)] = [2]int{uid, gid}
		return nil
	}
	t.Cleanup(func() { rootlessLchown = originalLchown })

	imageAnnotation, err := utils.JsonToString(spec.ImageConfigObject{
		RootfsType: "overlay",
		ImageLayer: []string{layerRootfs},
		UpperDir:   filepath.Join(root, "container", "diff"),
		WorkDir:    filepath.Join(root, "container", "work"),
	})
	require.NoError(t, err)
	containerSpec := spec.Spec{
		Annotations: spec.AnnotationObject{
			Image:    imageAnnotation,
			Rootless: `{"enabled":true}`,
		},
	}

	// == exercise ==
	updated, err := prepareRootlessShiftedImageLayers("container-1", containerSpec)

	// == assert ==
	require.NoError(t, err)
	var updatedImage spec.ImageConfigObject
	require.NoError(t, utils.StringToJson(updated.Annotations.Image, &updatedImage))
	require.Len(t, updatedImage.ImageLayer, 1)
	assert.Contains(t, updatedImage.ImageLayer[0], filepath.Join("rootless-shifted", "uid_200000_gid_300000_size_65536_v1", "rootfs"))
	assert.NotContains(t, updatedImage.ImageLayer[0], "container-1")

	shiftedFileSuffix := filepath.Join("rootfs", "var", "log", "app.log")
	shiftedFileChown, ok := findChownCallBySuffix(chownCalls, shiftedFileSuffix)
	require.True(t, ok, "expected chown call for shifted file suffix %q; calls=%v", shiftedFileSuffix, chownCalls)
	assert.Equal(t, [2]int{200000 + originalUID, 300000 + originalGID}, shiftedFileChown)
	_, err = os.Stat(rootlessShiftedLayerCompleteMarker(filepath.Dir(updatedImage.ImageLayer[0])))
	require.NoError(t, err)

	// Change the original after cache creation. A second prepare should reuse the
	// cache rather than copying the source layer again.
	require.NoError(t, os.WriteFile(filepath.Join(layerRootfs, "var", "log", "second.log"), []byte("new"), 0o644))
	updatedAgain, err := prepareRootlessShiftedImageLayers("container-2", containerSpec)
	require.NoError(t, err)
	var updatedAgainImage spec.ImageConfigObject
	require.NoError(t, utils.StringToJson(updatedAgain.Annotations.Image, &updatedAgainImage))
	assert.Equal(t, updatedImage.ImageLayer, updatedAgainImage.ImageLayer)
	_, err = os.Stat(filepath.Join(updatedAgainImage.ImageLayer[0], "var", "log", "second.log"))
	assert.True(t, os.IsNotExist(err), "existing cache should be reused without recopying the source layer")
}

func findChownCallBySuffix(calls map[string][2]int, suffix string) ([2]int, bool) {
	cleanSuffix := filepath.Clean(suffix)
	for path, ids := range calls {
		if strings.HasSuffix(filepath.Clean(path), cleanSuffix) {
			return ids, true
		}
	}
	return [2]int{}, false
}

func TestRootlessShiftedLayerCacheRootRequiresRootfsPath(t *testing.T) {
	_, err := rootlessShiftedLayerCacheRoot("/tmp/not-rootfs", 100000, 100000, 65536)

	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "rootfs path"))
}

func TestShiftRootlessIDsOffsetsArbitraryImageUsers(t *testing.T) {
	// This intentionally covers common service users from different images, not
	// just nginx's uid/gid 101.
	tests := []struct {
		name string
		uid  int
		gid  int
	}{
		{name: "root", uid: 0, gid: 0},
		{name: "www-data", uid: 33, gid: 33},
		{name: "apache", uid: 48, gid: 48},
		{name: "nginx", uid: 101, gid: 101},
		{name: "app user", uid: 1000, gid: 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uid, gid, err := shiftRootlessIDs("/rootfs/file", tt.uid, tt.gid, 100000, 200000, 65536)

			require.NoError(t, err)
			assert.Equal(t, 100000+tt.uid, uid)
			assert.Equal(t, 200000+tt.gid, gid)
		})
	}
}

func TestShiftRootlessIDsRejectsIDsOutsideMap(t *testing.T) {
	_, _, err := shiftRootlessIDs("/rootfs/high-uid", 65536, 0, 100000, 100000, 65536)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "uid outside rootless map")

	_, _, err = shiftRootlessIDs("/rootfs/high-gid", 0, 65536, 100000, 100000, 65536)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gid outside rootless map")
}

func TestRootlessShiftedLayerCacheReadyRequiresRootfsAndMarker(t *testing.T) {
	root := t.TempDir()
	cacheRoot := filepath.Join(root, "rootless-shifted", "uid_100000_gid_100000_size_65536_v1")
	rootfs := filepath.Join(cacheRoot, "rootfs")
	marker := rootlessShiftedLayerCompleteMarker(cacheRoot)

	assert.False(t, rootlessShiftedLayerCacheReady(rootfs, marker))
	require.NoError(t, os.MkdirAll(rootfs, 0o755))
	assert.False(t, rootlessShiftedLayerCacheReady(rootfs, marker))
	require.NoError(t, os.WriteFile(marker, []byte("complete\n"), 0o644))
	assert.True(t, rootlessShiftedLayerCacheReady(rootfs, marker))
}
