package env

import (
	"os"
	"path/filepath"
	"testing"

	"raind/internal/condenser/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigrateStoreFileMovesOldStoreToNestedPath(t *testing.T) {
	tmp := t.TempDir()
	oldPath := filepath.Join(tmp, "store", "psm.json")
	newPath := filepath.Join(tmp, "store", "resource", "pod", "psm.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(oldPath), 0o755))
	require.NoError(t, os.WriteFile(oldPath, []byte(`{"version":"1"}`), 0o644))

	manager := &BootstrapManager{filesystemHandler: utils.NewFilesystemExecutor()}
	err := manager.migrateStoreFile(storeMigration{Name: "psm", Old: oldPath, New: newPath})

	require.NoError(t, err)
	assert.NoFileExists(t, oldPath)
	require.FileExists(t, newPath)
	content, err := os.ReadFile(newPath)
	require.NoError(t, err)
	assert.JSONEq(t, `{"version":"1"}`, string(content))
}

func TestMigrateStoreFileKeepsOldStoreWhenNestedPathAlreadyExists(t *testing.T) {
	tmp := t.TempDir()
	oldPath := filepath.Join(tmp, "store", "psm.json")
	newPath := filepath.Join(tmp, "store", "resource", "pod", "psm.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(oldPath), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Dir(newPath), 0o755))
	require.NoError(t, os.WriteFile(oldPath, []byte(`{"old":true}`), 0o644))
	require.NoError(t, os.WriteFile(newPath, []byte(`{"new":true}`), 0o644))

	manager := &BootstrapManager{filesystemHandler: utils.NewFilesystemExecutor()}
	err := manager.migrateStoreFile(storeMigration{Name: "psm", Old: oldPath, New: newPath})

	require.NoError(t, err)
	require.FileExists(t, oldPath)
	require.FileExists(t, newPath)
	content, err := os.ReadFile(newPath)
	require.NoError(t, err)
	assert.JSONEq(t, `{"new":true}`, string(content))
}

func TestMigrateStoreFileIgnoresMissingOldStore(t *testing.T) {
	tmp := t.TempDir()
	oldPath := filepath.Join(tmp, "store", "missing.json")
	newPath := filepath.Join(tmp, "store", "resource", "pod", "psm.json")

	manager := &BootstrapManager{filesystemHandler: utils.NewFilesystemExecutor()}
	err := manager.migrateStoreFile(storeMigration{Name: "psm", Old: oldPath, New: newPath})

	require.NoError(t, err)
	assert.NoFileExists(t, oldPath)
	assert.NoFileExists(t, newPath)
}
