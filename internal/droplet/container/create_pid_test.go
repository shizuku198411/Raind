package container

import (
	"os"
	"path/filepath"
	"raind/internal/droplet/utils"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerCreatorTryReadPidFileReturnsValidPid(t *testing.T) {
	// == setup ==
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "init.pid")
	require.NoError(t, os.WriteFile(pidPath, []byte("4242\n"), 0644))
	creator := &ContainerCreator{}

	// == exercise ==
	pid, ok := creator.tryReadPidFile(pidPath)

	// == assert ==
	require.True(t, ok)
	assert.Equal(t, 4242, pid)
}

func TestContainerCreatorWaitInitPidReturnsExistingPidFile(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	require.NoError(t, os.MkdirAll(utils.ContainerDir(containerId), 0755))
	require.NoError(t, os.WriteFile(utils.InitPidFilePath(containerId), []byte("4242\n"), 0644))
	creator := &ContainerCreator{}

	// == exercise ==
	pid, err := creator.waitInitPid(containerId, time.Second, time.Millisecond)

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, 4242, pid)
}
