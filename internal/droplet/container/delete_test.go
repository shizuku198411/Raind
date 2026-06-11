package container

import (
	"errors"
	"os"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDeleteSpecLoader struct {
	spec   spec.Spec
	err    error
	calls  []string
	onLoad func(containerId string)
}

func (f *fakeDeleteSpecLoader) loadFile(containerId string) (spec.Spec, error) {
	f.calls = append(f.calls, containerId)
	if f.onLoad != nil {
		f.onLoad(containerId)
	}
	return f.spec, f.err
}

type fakeDeleteFifoHandler struct {
	err   error
	calls []string
}

func (f *fakeDeleteFifoHandler) removeFifo(path string) error {
	f.calls = append(f.calls, path)
	return f.err
}

type fakeDeleteStatusManager struct {
	status  status.ContainerStatus
	pid     int
	shimPid int

	getStatusErr  error
	getPidErr     error
	getShimPidErr error
	createErr     error
	updateErr     error
	removeErr     error

	createdStatuses   []deleteCreateStatusCall
	removedContainers []string
	updates           []deleteStatusUpdate
	reasonUpdates     []deleteReasonUpdate
}

type deleteCreateStatusCall struct {
	containerId string
	pid         int
	status      status.ContainerStatus
	rootfs      string
	bundle      string
	annotation  spec.AnnotationObject
}

type deleteStatusUpdate struct {
	containerId string
	status      status.ContainerStatus
	pid         int
	shimPid     int
}

type deleteReasonUpdate struct {
	containerId string
	reason      string
	message     string
}

func (f *fakeDeleteStatusManager) CreateStatusFile(containerId string, pid int, containerStatus status.ContainerStatus, rootfs string, bundle string, annotation spec.AnnotationObject) error {
	f.createdStatuses = append(f.createdStatuses, deleteCreateStatusCall{
		containerId: containerId,
		pid:         pid,
		status:      containerStatus,
		rootfs:      rootfs,
		bundle:      bundle,
		annotation:  annotation,
	})
	return f.createErr
}

func (f *fakeDeleteStatusManager) RemoveStatusFile(containerId string) error {
	f.removedContainers = append(f.removedContainers, containerId)
	return f.removeErr
}

func (f *fakeDeleteStatusManager) ReadStatusFile(string) (string, error) {
	return "", nil
}

func (f *fakeDeleteStatusManager) UpdateStatus(containerId string, containerStatus status.ContainerStatus, pid int, shimPid int) error {
	f.updates = append(f.updates, deleteStatusUpdate{
		containerId: containerId,
		status:      containerStatus,
		pid:         pid,
		shimPid:     shimPid,
	})
	return f.updateErr
}

func (f *fakeDeleteStatusManager) UpdateExitCode(string, int) error {
	return nil
}

func (f *fakeDeleteStatusManager) UpdateReasonAndMessage(containerId string, reason string, message string) error {
	f.reasonUpdates = append(f.reasonUpdates, deleteReasonUpdate{
		containerId: containerId,
		reason:      reason,
		message:     message,
	})
	return nil
}

func (f *fakeDeleteStatusManager) GetPidFromId(string) (int, error) {
	return f.pid, f.getPidErr
}

func (f *fakeDeleteStatusManager) GetStatusFromId(string) (status.ContainerStatus, error) {
	return f.status, f.getStatusErr
}

func (f *fakeDeleteStatusManager) GetShimPidFromId(string) (int, error) {
	return f.shimPid, f.getShimPidErr
}

func (f *fakeDeleteStatusManager) ListContainers() ([]status.StatusObject, error) {
	return nil, nil
}

type fakeDeleteHookController struct {
	createRuntimeErr     error
	createContainerErr   error
	startContainerErr    error
	poststartErr         error
	stopContainerErr     error
	poststopErr          error
	createRuntimeCalls   []deleteHookCall
	createContainerCalls []deleteHookCall
	startContainerCalls  []deleteHookCall
	poststartCalls       []deleteHookCall
	stopContainerCalls   []deleteHookCall
	poststopCalls        []deleteHookCall
}

type deleteHookCall struct {
	containerId string
	hooks       []spec.HookObject
}

func (f *fakeDeleteHookController) RunCreateRuntimeHooks(containerId string, hooks []spec.HookObject) error {
	f.createRuntimeCalls = append(f.createRuntimeCalls, deleteHookCall{
		containerId: containerId,
		hooks:       hooks,
	})
	return f.createRuntimeErr
}

func (f *fakeDeleteHookController) RunCreateContainerHooks(containerId string, hooks []spec.HookObject) error {
	f.createContainerCalls = append(f.createContainerCalls, deleteHookCall{
		containerId: containerId,
		hooks:       hooks,
	})
	return f.createContainerErr
}

func (f *fakeDeleteHookController) RunStartContainerHooks(containerId string, hooks []spec.HookObject) error {
	f.startContainerCalls = append(f.startContainerCalls, deleteHookCall{
		containerId: containerId,
		hooks:       hooks,
	})
	return f.startContainerErr
}

func (f *fakeDeleteHookController) RunPoststartHooks(containerId string, hooks []spec.HookObject) error {
	f.poststartCalls = append(f.poststartCalls, deleteHookCall{
		containerId: containerId,
		hooks:       hooks,
	})
	return f.poststartErr
}

func (f *fakeDeleteHookController) RunStopContainerHooks(containerId string, hooks []spec.HookObject) error {
	f.stopContainerCalls = append(f.stopContainerCalls, deleteHookCall{
		containerId: containerId,
		hooks:       hooks,
	})
	return f.stopContainerErr
}

func (f *fakeDeleteHookController) RunPoststopHooks(containerId string, hooks []spec.HookObject) error {
	f.poststopCalls = append(f.poststopCalls, deleteHookCall{
		containerId: containerId,
		hooks:       hooks,
	})
	return f.poststopErr
}

type fakeDeleteSyscallHandler struct {
	utils.KernelSyscallHandler

	killErr   error
	kills     []deleteKillCall
	removeErr error
	removes   []string
	openFiles []string
}

type deleteKillCall struct {
	pid int
	sig syscall.Signal
}

func (f *fakeDeleteSyscallHandler) Kill(pid int, sig syscall.Signal) error {
	f.kills = append(f.kills, deleteKillCall{pid: pid, sig: sig})
	return f.killErr
}

func (f *fakeDeleteSyscallHandler) Remove(path string) error {
	f.removes = append(f.removes, path)
	return f.removeErr
}

func (f *fakeDeleteSyscallHandler) OpenFile(path string, flag int, perm os.FileMode) (*os.File, error) {
	f.openFiles = append(f.openFiles, path)
	tmp, err := os.CreateTemp("", "raind-test-open-file-*")
	if err != nil {
		return nil, err
	}
	return tmp, nil
}

func TestContainerDeleteRejectsRunningContainer(t *testing.T) {
	// == setup ==
	specLoader := &fakeDeleteSpecLoader{}
	fifoHandler := &fakeDeleteFifoHandler{}
	statusManager := &fakeDeleteStatusManager{status: status.RUNNING}
	hookController := &fakeDeleteHookController{}
	syscalls := &fakeDeleteSyscallHandler{}
	deleteController := &ContainerDelete{
		specLoader:              specLoader,
		fifoHandler:             fifoHandler,
		containerStatusManager:  statusManager,
		containerHookController: hookController,
		syscallHandler:          syscalls,
	}

	// == exercise ==
	err := deleteController.Delete(DeleteOption{ContainerId: "container-1"})

	// == assert ==
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not stopped")
	assert.Empty(t, specLoader.calls)
	assert.Empty(t, hookController.poststopCalls)
	assert.Empty(t, statusManager.removedContainers)
	assert.Empty(t, fifoHandler.calls)
	assert.Empty(t, syscalls.kills)
}

func TestContainerDeleteRemovesStoppedContainerState(t *testing.T) {
	// == setup ==
	poststopHooks := []spec.HookObject{{Path: "/bin/true"}}
	specLoader := &fakeDeleteSpecLoader{
		spec: spec.Spec{
			Hooks: spec.HookLifecycleObject{Poststop: poststopHooks},
		},
	}
	fifoHandler := &fakeDeleteFifoHandler{}
	statusManager := &fakeDeleteStatusManager{status: status.STOPPED}
	hookController := &fakeDeleteHookController{}
	syscalls := &fakeDeleteSyscallHandler{}
	deleteController := &ContainerDelete{
		specLoader:              specLoader,
		fifoHandler:             fifoHandler,
		containerStatusManager:  statusManager,
		containerHookController: hookController,
		syscallHandler:          syscalls,
	}

	// == exercise ==
	err := deleteController.Delete(DeleteOption{ContainerId: "container-1"})

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, []string{"container-1"}, specLoader.calls)
	assert.Equal(t, []deleteHookCall{{containerId: "container-1", hooks: poststopHooks}}, hookController.poststopCalls)
	assert.Equal(t, []string{"container-1"}, statusManager.removedContainers)
	assert.Empty(t, fifoHandler.calls)
	assert.Empty(t, syscalls.kills)
}

func TestContainerDeleteKillsCreatedContainerInitAndRemovesFifo(t *testing.T) {
	// == setup ==
	specLoader := &fakeDeleteSpecLoader{}
	fifoHandler := &fakeDeleteFifoHandler{}
	statusManager := &fakeDeleteStatusManager{status: status.CREATED, pid: 1234}
	hookController := &fakeDeleteHookController{}
	syscalls := &fakeDeleteSyscallHandler{}
	deleteController := &ContainerDelete{
		specLoader:              specLoader,
		fifoHandler:             fifoHandler,
		containerStatusManager:  statusManager,
		containerHookController: hookController,
		syscallHandler:          syscalls,
	}

	// == exercise ==
	err := deleteController.Delete(DeleteOption{ContainerId: "container-1"})

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, []deleteKillCall{{pid: 1234, sig: syscall.SIGKILL}}, syscalls.kills)
	assert.Equal(t, []deleteStatusUpdate{{
		containerId: "container-1",
		status:      status.STOPPED,
		pid:         0,
		shimPid:     0,
	}}, statusManager.updates)
	assert.Equal(t, []string{"container-1"}, statusManager.removedContainers)
	assert.Equal(t, []string{utils.FifoPath("container-1")}, fifoHandler.calls)
}

func TestContainerDeleteStopsWhenPoststopHookFails(t *testing.T) {
	// == setup ==
	specLoader := &fakeDeleteSpecLoader{}
	fifoHandler := &fakeDeleteFifoHandler{}
	statusManager := &fakeDeleteStatusManager{status: status.STOPPED}
	hookController := &fakeDeleteHookController{poststopErr: errors.New("hook failed")}
	syscalls := &fakeDeleteSyscallHandler{}
	deleteController := &ContainerDelete{
		specLoader:              specLoader,
		fifoHandler:             fifoHandler,
		containerStatusManager:  statusManager,
		containerHookController: hookController,
		syscallHandler:          syscalls,
	}

	// == exercise ==
	err := deleteController.Delete(DeleteOption{ContainerId: "container-1"})

	// == assert ==
	require.Error(t, err)
	assert.Equal(t, "hook failed", err.Error())
	assert.Equal(t, []string{"container-1"}, specLoader.calls)
	assert.Len(t, hookController.poststopCalls, 1)
	assert.Empty(t, statusManager.removedContainers)
	assert.Empty(t, fifoHandler.calls)
}
