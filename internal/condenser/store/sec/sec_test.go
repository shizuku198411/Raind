package sec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretStoreUsesRestrictedPermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store", "resource", "secret", "sec.json")
	manager := NewSecManager(NewSecStore(path))

	err := manager.StoreSecret("secret-1", SecretInfo{
		Name:      "db-secret",
		Namespace: "demo",
		Type:      SecretTypeOpaque,
		Data:      map[string]string{"DB_PASSWORD": "super-secret"},
	})

	require.NoError(t, err)
	fileInfo, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm())

	dirInfo, err := os.Stat(filepath.Dir(path))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())
}
