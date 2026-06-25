package container

import (
	"os"
	"path/filepath"
	"testing"

	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareBundleConfigCopiesConfigAndResolvesRelativeRoot(t *testing.T) {
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	bundle := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(bundle, "rootfs"), 0755))
	require.NoError(t, utils.WriteJsonToFile(filepath.Join(bundle, "config.json"), spec.Spec{
		OciVersion: "1.3.0",
		Root:       spec.RootObject{Path: "rootfs", Readonly: true},
		Process: spec.ProcessObject{
			Cwd:  "/",
			Args: []string{"/bin/sh"},
			Env:  []string{"PATH=/bin"},
		},
	}))

	err := prepareBundleConfig(containerId, bundle)

	require.NoError(t, err)
	var got spec.Spec
	require.NoError(t, utils.ReadJsonFile(utils.ConfigFilePath(containerId), &got))
	assert.Equal(t, filepath.Join(bundle, "rootfs"), got.Root.Path)
	assert.True(t, got.Root.Readonly)
	assert.DirExists(t, filepath.Join(utils.ContainerDir(containerId), "logs"))
}

func TestPrepareBundleConfigKeepsAbsoluteRoot(t *testing.T) {
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	bundle := t.TempDir()
	rootfs := filepath.Join(t.TempDir(), "rootfs")
	require.NoError(t, os.MkdirAll(rootfs, 0755))
	require.NoError(t, utils.WriteJsonToFile(filepath.Join(bundle, "config.json"), spec.Spec{
		OciVersion: "1.3.0",
		Root:       spec.RootObject{Path: rootfs},
		Process: spec.ProcessObject{
			Args: []string{"/bin/sh"},
		},
	}))

	err := prepareBundleConfig(containerId, bundle)

	require.NoError(t, err)
	var got spec.Spec
	require.NoError(t, utils.ReadJsonFile(utils.ConfigFilePath(containerId), &got))
	assert.Equal(t, rootfs, got.Root.Path)
}

func TestWriteContainerPidFile(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "nested", "container.pid")

	err := writeContainerPidFile(pidFile, 1234)

	require.NoError(t, err)
	data, err := os.ReadFile(pidFile)
	require.NoError(t, err)
	assert.Equal(t, "1234\n", string(data))
}
