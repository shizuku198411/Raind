package container

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

	shiftedFile := filepath.Join(updatedImage.ImageLayer[0], "var", "log", "app.log")
	info, err := os.Lstat(shiftedFile)
	require.NoError(t, err)
	st := info.Sys().(*syscall.Stat_t)
	assert.Equal(t, uint32(200000+originalUID), st.Uid)
	assert.Equal(t, uint32(300000+originalGID), st.Gid)
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

func TestRootlessShiftedLayerCacheRootRequiresRootfsPath(t *testing.T) {
	_, err := rootlessShiftedLayerCacheRoot("/tmp/not-rootfs", 100000, 100000, 65536)

	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "rootfs path"))
}
