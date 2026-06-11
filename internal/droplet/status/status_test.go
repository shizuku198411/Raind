package status

import (
	"os"
	"path/filepath"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStatusSyscallHandler struct {
	utils.KernelSyscallHandler

	killErr error
	kills   []int
	removed []string
}

func (f *fakeStatusSyscallHandler) Kill(pid int, sig syscall.Signal) error {
	f.kills = append(f.kills, pid)
	return f.killErr
}

func (f *fakeStatusSyscallHandler) Remove(name string) error {
	f.removed = append(f.removed, name)
	return os.Remove(name)
}

func (f *fakeStatusSyscallHandler) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}

func TestContainerStatusStringAndParse(t *testing.T) {
	// == assert ==
	assert.Equal(t, "creating", CREATING.String())
	assert.Equal(t, "created", CREATED.String())
	assert.Equal(t, "running", RUNNING.String())
	assert.Equal(t, "stopped", STOPPED.String())
	assert.Equal(t, "unknown", ContainerStatus(99).String())

	got, err := ParseContainerStatus("RUNNING")
	require.NoError(t, err)
	assert.Equal(t, RUNNING, got)
	_, err = ParseContainerStatus("bad")
	require.Error(t, err)
}

func TestStatusHandlerCreateReadAndUpdateStatusFile(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	require.NoError(t, os.MkdirAll(utils.ContainerDir(containerId), 0755))
	handler := &StatusHandler{syscallHandler: &fakeStatusSyscallHandler{}}
	annotation := spec.AnnotationObject{Version: "v1", Net: "{}", Image: "{}"}

	// == exercise ==
	require.NoError(t, handler.CreateStatusFile(containerId, 123, RUNNING, "/rootfs", "/bundle", annotation))
	require.NoError(t, handler.UpdateStatus(containerId, STOPPED, 0, -1))
	require.NoError(t, handler.UpdateExitCode(containerId, 7))
	require.NoError(t, handler.UpdateReasonAndMessage(containerId, "Error", "failed"))
	pid, err := handler.GetPidFromId(containerId)
	require.NoError(t, err)
	shimPid, err := handler.GetShimPidFromId(containerId)
	require.NoError(t, err)
	containerStatus, err := handler.GetStatusFromId(containerId)
	require.NoError(t, err)

	// == assert ==
	assert.Equal(t, 0, pid)
	assert.Equal(t, 0, shimPid)
	assert.Equal(t, STOPPED, containerStatus)
	var state StatusObject
	require.NoError(t, utils.ReadJsonFile(utils.ContainerStatePath(containerId), &state))
	assert.Equal(t, containerId, state.Id)
	assert.Equal(t, "stopped", state.Status)
	assert.Equal(t, 7, state.ExitCode)
	assert.Equal(t, "Error", state.Reason)
	assert.Equal(t, "failed", state.Message)
	assert.Equal(t, annotation, state.Annotaion)
}

func TestStatusHandlerRecomputesRunningDeadPidToStopped(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	require.NoError(t, os.MkdirAll(utils.ContainerDir(containerId), 0755))
	handler := &StatusHandler{syscallHandler: &fakeStatusSyscallHandler{killErr: syscall.ESRCH}}
	require.NoError(t, handler.CreateStatusFile(containerId, 99999, RUNNING, "/rootfs", "/bundle", spec.AnnotationObject{}))

	// == exercise ==
	containerStatus, err := handler.GetStatusFromId(containerId)

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, STOPPED, containerStatus)
	assert.Equal(t, 0, mustPidFromStatus(t, handler, containerId))
}

func TestStatusHandlerPidAliveCases(t *testing.T) {
	tests := []struct {
		name  string
		pid   int
		err   error
		alive bool
	}{
		{name: "invalid pid", pid: 0, alive: false},
		{name: "alive", pid: 123, alive: true},
		{name: "missing", pid: 123, err: syscall.ESRCH, alive: false},
		{name: "permission", pid: 123, err: syscall.EPERM, alive: true},
		{name: "other error", pid: 123, err: syscall.EINVAL, alive: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// == setup ==
			handler := &StatusHandler{syscallHandler: &fakeStatusSyscallHandler{killErr: tt.err}}

			// == exercise ==
			alive, err := handler.pidAlive(tt.pid)

			// == assert ==
			require.NoError(t, err)
			assert.Equal(t, tt.alive, alive)
		})
	}
}

func TestStatusHandlerListContainersSkipsMissingStateAndRecomputes(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	handler := &StatusHandler{syscallHandler: &fakeStatusSyscallHandler{killErr: syscall.ESRCH}}
	require.NoError(t, os.MkdirAll(utils.ContainerDir("alive"), 0755))
	require.NoError(t, os.MkdirAll(utils.ContainerDir("missing-state"), 0755))
	require.NoError(t, handler.CreateStatusFile("alive", 99999, RUNNING, "/rootfs", "/bundle", spec.AnnotationObject{}))
	require.NoError(t, os.WriteFile(filepath.Join(rootDir, "not-dir"), []byte("x"), 0644))

	// == exercise ==
	containers, err := handler.ListContainers()

	// == assert ==
	require.NoError(t, err)
	require.Len(t, containers, 1)
	assert.Equal(t, "alive", containers[0].Id)
	assert.Equal(t, "stopped", containers[0].Status)
}

func TestStatusHandlerRemoveStatusFile(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	require.NoError(t, os.MkdirAll(utils.ContainerDir(containerId), 0755))
	syscalls := &fakeStatusSyscallHandler{}
	handler := &StatusHandler{syscallHandler: syscalls}
	require.NoError(t, handler.CreateStatusFile(containerId, 0, CREATED, "/rootfs", "/bundle", spec.AnnotationObject{}))

	// == exercise ==
	err := handler.RemoveStatusFile(containerId)

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, []string{utils.ContainerStatePath(containerId)}, syscalls.removed)
	assert.NoFileExists(t, utils.ContainerStatePath(containerId))
}

func mustPidFromStatus(t *testing.T, handler *StatusHandler, containerId string) int {
	t.Helper()
	pid, err := handler.GetPidFromId(containerId)
	require.NoError(t, err)
	return pid
}
