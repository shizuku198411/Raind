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

func TestResolveBuildFileRejectsPathEscapingContext(t *testing.T) {
	dir := t.TempDir()
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "Dripfile")
	rel, err := filepath.Rel(dir, outside)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(outside, []byte("FROM scratch\n"), 0o644))

	_, err = resolveBuildFile(dir, rel)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes build context")
}

func TestResolveBuildFileRejectsAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "Dripfile")
	require.NoError(t, os.WriteFile(outside, []byte("FROM scratch\n"), 0o644))

	_, err := resolveBuildFile(dir, outside)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be relative")
}

func TestResolveBuildFileNormalizesExplicitFileInsideContext(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "subdir"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Dripfile"), []byte("FROM scratch\n"), 0o644))

	got, err := resolveBuildFile(dir, filepath.Join("subdir", "..", "Dripfile"))

	require.NoError(t, err)
	assert.Equal(t, "Dripfile", got)
}
