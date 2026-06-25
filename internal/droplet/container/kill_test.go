package container

import (
	"context"
	"os"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerKillSendsNonStoppingSignalWithoutStopHook(t *testing.T) {
	// == setup ==
	stopHooks := []spec.HookObject{{Path: "/bin/stop"}}
	containerSpec := spec.Spec{
		Hooks: spec.HookLifecycleObject{StopContainer: stopHooks},
		Process: spec.ProcessObject{
			Args: []string{"/bin/sh"},
		},
		LinuxSpec: spec.LinuxSpecObject{
			Seccomp: &spec.SeccompObject{DefaultAction: "SCMP_ACT_ALLOW"},
		},
	}
	specLoader := &fakeDeleteSpecLoader{spec: containerSpec}
	statusManager := &fakeDeleteStatusManager{status: status.RUNNING, pid: os.Getpid()}
	hookController := &fakeDeleteHookController{}
	syscalls := &fakeDeleteSyscallHandler{}
	killController := &ContainerKill{
		specLoader:              specLoader,
		syscallHandler:          syscalls,
		containerStatusManager:  statusManager,
		containerHookController: hookController,
	}

	// == exercise ==
	err := killController.Kill(KillOption{ContainerId: "container-1", Signal: "CONT"})

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, []string{"container-1"}, specLoader.calls)
	assert.Equal(t, []deleteKillCall{{pid: os.Getpid(), sig: syscall.SIGCONT}}, syscalls.kills)
	assert.Equal(t, []deleteStatusUpdate{{containerId: "container-1", status: status.RUNNING, pid: -1, shimPid: -1}}, statusManager.updates)
	assert.Empty(t, hookController.stopContainerCalls)
}

func TestContainerKillAllowsCreatedContainerSignalAndRunsStopHook(t *testing.T) {
	// == setup ==
	stopHooks := []spec.HookObject{{Path: "/bin/stop"}}
	containerSpec := spec.Spec{
		Hooks:   spec.HookLifecycleObject{StopContainer: stopHooks},
		Process: spec.ProcessObject{Args: []string{"/bin/sh"}},
	}
	specLoader := &fakeDeleteSpecLoader{spec: containerSpec}
	statusManager := &fakeDeleteStatusManager{status: status.CREATED, pid: os.Getpid()}
	hookController := &fakeDeleteHookController{}
	syscalls := &fakeDeleteSyscallHandler{}
	killController := &ContainerKill{
		specLoader:              specLoader,
		syscallHandler:          syscalls,
		containerStatusManager:  statusManager,
		containerHookController: hookController,
	}

	// == exercise ==
	err := killController.Kill(KillOption{ContainerId: "container-1", Signal: "CONT"})

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, []deleteKillCall{{pid: os.Getpid(), sig: syscall.SIGCONT}}, syscalls.kills)
	assert.Equal(t, []deleteStatusUpdate{{containerId: "container-1", status: status.STOPPED, pid: 0, shimPid: 0}}, statusManager.updates)
	assert.Equal(t, []deleteHookCall{{containerId: "container-1", hooks: stopHooks}}, hookController.stopContainerCalls)
}

func TestContainerKillAcceptsSigPrefixSignal(t *testing.T) {
	// == setup ==
	killController := &ContainerKill{
		specLoader:              &fakeDeleteSpecLoader{spec: spec.Spec{Process: spec.ProcessObject{Args: []string{"/bin/sh"}}}},
		syscallHandler:          &fakeDeleteSyscallHandler{},
		containerStatusManager:  &fakeDeleteStatusManager{status: status.RUNNING, pid: os.Getpid()},
		containerHookController: &fakeDeleteHookController{},
	}

	// == exercise ==
	err := killController.Kill(KillOption{ContainerId: "container-1", Signal: "SIGCONT"})

	// == assert ==
	require.NoError(t, err)
	syscalls := killController.syscallHandler.(*fakeDeleteSyscallHandler)
	assert.Equal(t, []deleteKillCall{{pid: os.Getpid(), sig: syscall.SIGCONT}}, syscalls.kills)
}

func TestParseSignalAcceptsNumericSignal(t *testing.T) {
	name, sig, err := parseSignal("15")

	require.NoError(t, err)
	assert.Equal(t, "15", name)
	assert.Equal(t, syscall.SIGTERM, sig)
}

func TestParseSignalRejectsUnsupportedSignal(t *testing.T) {
	_, _, err := parseSignal("SIGWAT")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported signal")
}

func TestContainerKillRejectsNonRunningContainer(t *testing.T) {
	// == setup ==
	specLoader := &fakeDeleteSpecLoader{}
	statusManager := &fakeDeleteStatusManager{status: status.STOPPED}
	syscalls := &fakeDeleteSyscallHandler{}
	killController := &ContainerKill{
		specLoader:              specLoader,
		syscallHandler:          syscalls,
		containerStatusManager:  statusManager,
		containerHookController: &fakeDeleteHookController{},
	}

	// == exercise ==
	err := killController.Kill(KillOption{ContainerId: "container-1", Signal: "KILL"})

	// == assert ==
	require.Error(t, err)
	assert.Contains(t, err.Error(), "neither created nor running")
	assert.Empty(t, syscalls.kills)
	assert.Empty(t, statusManager.updates)
}

func TestContainerKillAllowsStoppedKillCleanupForExternalPidFileContainer(t *testing.T) {
	// == setup ==
	containerId := "container-1"
	specLoader := &fakeDeleteSpecLoader{}
	statusManager := &fakeDeleteStatusManager{status: status.STOPPED}
	syscalls := &fakeDeleteSyscallHandler{
		existing: map[string]bool{utils.ExternalPidFileMarkerPath(containerId): true},
	}
	killController := &ContainerKill{
		specLoader:              specLoader,
		syscallHandler:          syscalls,
		containerStatusManager:  statusManager,
		containerHookController: &fakeDeleteHookController{},
	}

	// == exercise ==
	err := killController.Kill(KillOption{ContainerId: containerId, Signal: "KILL"})

	// == assert ==
	require.NoError(t, err)
	assert.Empty(t, syscalls.kills)
	assert.Empty(t, statusManager.updates)
}

func TestContainerKillCleanupShimRemovesSocketAndPidFile(t *testing.T) {
	// == setup ==
	syscalls := &fakeDeleteSyscallHandler{}
	killController := &ContainerKill{syscallHandler: syscalls}

	// == exercise ==
	err := killController.cleanupShim("container-1")

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, []string{
		utils.SockPath("container-1"),
		utils.InitPidFilePath("container-1"),
	}, syscalls.removes)
}

func TestContainerKillReadProcStartTimeReadsCurrentProcess(t *testing.T) {
	// == setup ==
	killController := &ContainerKill{}

	// == exercise ==
	startTime, err := killController.readProcStartTime(os.Getpid())

	// == assert ==
	require.NoError(t, err)
	assert.Greater(t, startTime, uint64(0))
}

func TestParseProcStatFieldsHandlesCommandWithSpaces(t *testing.T) {
	// == exercise ==
	fields, err := parseProcStatFields("123 (sleep worker) Z 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22")

	// == assert ==
	require.NoError(t, err)
	require.Len(t, fields, 23)
	assert.Equal(t, "Z", fields[0])
	assert.Equal(t, "19", fields[19])
}

func TestContainerKillWaitProcessExitReturnsWhenPidDoesNotExist(t *testing.T) {
	// == setup ==
	killController := &ContainerKill{}

	// == exercise ==
	err := killController.waitProcessExit(ProcIdentity{Pid: -1, StartTime: 1}, time.Second)

	// == assert ==
	require.NoError(t, err)
}

func TestContainerKillWaitProcessExitTimesOutForCurrentProcess(t *testing.T) {
	// == setup ==
	killController := &ContainerKill{}
	startTime, err := killController.readProcStartTime(os.Getpid())
	require.NoError(t, err)

	// == exercise ==
	err = killController.waitProcessExit(ProcIdentity{Pid: os.Getpid(), StartTime: startTime}, time.Millisecond)

	// == assert ==
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
