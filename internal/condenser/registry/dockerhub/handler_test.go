package dockerhub

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJoinRootReturnsAbsolutePathInsideRoot(t *testing.T) {
	rootfs := t.TempDir()
	registry := &RegistryDockerHub{}

	got, err := registry.joinRoot(rootfs, "etc/passwd")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(rootfs, "etc", "passwd"), got)
}

func TestJoinRootDeanchorsAbsoluteArchivePath(t *testing.T) {
	rootfs := t.TempDir()
	registry := &RegistryDockerHub{}

	got, err := registry.joinRoot(rootfs, "/etc/passwd")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(rootfs, "etc", "passwd"), got)
}

func TestJoinRootReturnsRootForCurrentDirectory(t *testing.T) {
	rootfs := t.TempDir()
	registry := &RegistryDockerHub{}

	got, err := registry.joinRoot(rootfs, ".")

	require.NoError(t, err)
	assert.Equal(t, rootfs, got)
}

func TestJoinRootRejectsEscapingPaths(t *testing.T) {
	rootfs := t.TempDir()
	registry := &RegistryDockerHub{}

	tests := []string{
		"../escape",
		"../../escape",
		"foo/../../../escape",
	}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			got, err := registry.joinRoot(rootfs, tt)

			require.Error(t, err)
			assert.Empty(t, got)
			assert.True(t, strings.Contains(err.Error(), "path escapes root"))
		})
	}
}
