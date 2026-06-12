package image

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveBuildFilePrefersDripfileThenDockerfile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte("FROM scratch\n"), 0o644))

	got, err := resolveBuildFile(dir, "")

	require.NoError(t, err)
	assert.Equal(t, "Dockerfile", got)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "Dripfile"), []byte("FROM scratch\n"), 0o644))

	got, err = resolveBuildFile(dir, "")

	require.NoError(t, err)
	assert.Equal(t, "Dripfile", got)
}

func TestResolveBuildFileUsesExplicitFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "custom.Dockerfile"), []byte("FROM scratch\n"), 0o644))

	got, err := resolveBuildFile(dir, "custom.Dockerfile")

	require.NoError(t, err)
	assert.Equal(t, "custom.Dockerfile", got)
}
