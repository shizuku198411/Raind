package spechash

import (
	"os"
	"testing"

	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLoader struct {
	spec  spec.Spec
	calls []string
}

func (f *fakeLoader) load(containerId string) (spec.Spec, error) {
	f.calls = append(f.calls, containerId)
	return f.spec, nil
}

func TestVerifyAndLoadValidatesHashBeforeLoading(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	containerSpec := spec.Spec{OciVersion: "1.0.2"}
	writeConfigForTest(t, containerId, []byte(`{"version":"1.0.2"}`))
	require.NoError(t, WriteCurrent(containerId))
	loader := &fakeLoader{spec: containerSpec}

	// == exercise ==
	got, err := VerifyAndLoad(containerId, loader.load)

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, containerSpec, got)
	assert.Equal(t, []string{containerId}, loader.calls)
}

func TestVerifyAndLoadDoesNotLoadWhenHashMismatch(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	writeConfigForTest(t, containerId, []byte(`{"version":"1.0.2"}`))
	require.NoError(t, utils.WriteJsonToFile(utils.ConfigFileHashPath(containerId), spec.SpecHash{Sha256: "stale"}))
	loader := &fakeLoader{spec: spec.Spec{OciVersion: "1.0.2"}}

	// == exercise ==
	_, err := VerifyAndLoad(containerId, loader.load)

	// == assert ==
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hash validation failed")
	assert.Empty(t, loader.calls)
}

func writeConfigForTest(t *testing.T, containerId string, data []byte) {
	t.Helper()
	require.NoError(t, os.MkdirAll(utils.ContainerDir(containerId), 0755))
	require.NoError(t, os.WriteFile(utils.ConfigFilePath(containerId), data, 0644))
}
