package container

import (
	"errors"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStartFifoHandler struct {
	writes    []string
	removes   []string
	writeErr  error
	removeErr error
}

func (f *fakeStartFifoHandler) writeFifo(path string) error {
	f.writes = append(f.writes, path)
	return f.writeErr
}

func (f *fakeStartFifoHandler) removeFifo(path string) error {
	f.removes = append(f.removes, path)
	return f.removeErr
}

func TestContainerStartExecuteRunsHooksSignalsFifoAndUpdatesState(t *testing.T) {
	// == setup ==
	startHooks := []spec.HookObject{{Path: "/bin/start"}}
	poststartHooks := []spec.HookObject{{Path: "/bin/poststart"}}
	specLoader := &fakeDeleteSpecLoader{
		spec: spec.Spec{
			Hooks: spec.HookLifecycleObject{
				StartContainer: startHooks,
				Poststart:      poststartHooks,
			},
		},
	}
	fifoHandler := &fakeStartFifoHandler{}
	statusManager := &fakeDeleteStatusManager{status: status.CREATED}
	hookController := &fakeDeleteHookController{}
	startController := &ContainerStart{
		specLoader:              specLoader,
		fifoHandler:             fifoHandler,
		containerStatusManager:  statusManager,
		containerHookController: hookController,
	}

	// == exercise ==
	err := startController.Execute(StartOption{ContainerId: "container-1"})

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, []string{"container-1"}, specLoader.calls)
	assert.Equal(t, []deleteHookCall{{containerId: "container-1", hooks: startHooks}}, hookController.startContainerCalls)
	assert.Equal(t, []string{utils.FifoPath("container-1")}, fifoHandler.writes)
	assert.Equal(t, []string{utils.FifoPath("container-1")}, fifoHandler.removes)
	assert.Equal(t, []deleteStatusUpdate{{
		containerId: "container-1",
		status:      status.RUNNING,
		pid:         -1,
		shimPid:     -1,
	}}, statusManager.updates)
	assert.Equal(t, []deleteHookCall{{containerId: "container-1", hooks: poststartHooks}}, hookController.poststartCalls)
}

func TestContainerStartExecuteRejectsNonCreatedContainer(t *testing.T) {
	// == setup ==
	specLoader := &fakeDeleteSpecLoader{}
	fifoHandler := &fakeStartFifoHandler{}
	statusManager := &fakeDeleteStatusManager{status: status.STOPPED}
	hookController := &fakeDeleteHookController{}
	startController := &ContainerStart{
		specLoader:              specLoader,
		fifoHandler:             fifoHandler,
		containerStatusManager:  statusManager,
		containerHookController: hookController,
	}

	// == exercise ==
	err := startController.Execute(StartOption{ContainerId: "container-1"})

	// == assert ==
	require.Error(t, err)
	assert.Contains(t, err.Error(), "is not created")
	assert.Empty(t, specLoader.calls)
	assert.Empty(t, fifoHandler.writes)
	assert.Empty(t, statusManager.updates)
}

func TestContainerStartExecuteStopsWhenStartHookFails(t *testing.T) {
	// == setup ==
	specLoader := &fakeDeleteSpecLoader{spec: spec.Spec{
		Hooks: spec.HookLifecycleObject{StartContainer: []spec.HookObject{{Path: "/bin/start"}}},
	}}
	fifoHandler := &fakeStartFifoHandler{}
	statusManager := &fakeDeleteStatusManager{status: status.CREATED}
	hookController := &fakeDeleteHookController{startContainerErr: errors.New("hook failed")}
	startController := &ContainerStart{
		specLoader:              specLoader,
		fifoHandler:             fifoHandler,
		containerStatusManager:  statusManager,
		containerHookController: hookController,
	}

	// == exercise ==
	err := startController.Execute(StartOption{ContainerId: "container-1"})

	// == assert ==
	require.Error(t, err)
	assert.Equal(t, "hook failed", err.Error())
	assert.Empty(t, fifoHandler.writes)
	assert.Empty(t, statusManager.updates)
}

func TestContainerStartExecuteStopsWhenFifoWriteFails(t *testing.T) {
	// == setup ==
	specLoader := &fakeDeleteSpecLoader{}
	fifoHandler := &fakeStartFifoHandler{writeErr: errors.New("write failed")}
	statusManager := &fakeDeleteStatusManager{status: status.CREATED}
	startController := &ContainerStart{
		specLoader:              specLoader,
		fifoHandler:             fifoHandler,
		containerStatusManager:  statusManager,
		containerHookController: &fakeDeleteHookController{},
	}

	// == exercise ==
	err := startController.Execute(StartOption{ContainerId: "container-1"})

	// == assert ==
	require.Error(t, err)
	assert.Equal(t, "write failed", err.Error())
	assert.Empty(t, fifoHandler.removes)
	assert.Empty(t, statusManager.updates)
}

func TestContainerStartExecuteReturnsPoststartErrorAfterRunningState(t *testing.T) {
	// == setup ==
	specLoader := &fakeDeleteSpecLoader{spec: spec.Spec{
		Hooks: spec.HookLifecycleObject{Poststart: []spec.HookObject{{Path: "/bin/poststart"}}},
	}}
	fifoHandler := &fakeStartFifoHandler{}
	statusManager := &fakeDeleteStatusManager{status: status.CREATED}
	hookController := &fakeDeleteHookController{poststartErr: errors.New("poststart failed")}
	startController := &ContainerStart{
		specLoader:              specLoader,
		fifoHandler:             fifoHandler,
		containerStatusManager:  statusManager,
		containerHookController: hookController,
	}

	// == exercise ==
	err := startController.Execute(StartOption{ContainerId: "container-1"})

	// == assert ==
	require.Error(t, err)
	assert.Equal(t, "poststart failed", err.Error())
	assert.Equal(t, []deleteStatusUpdate{{containerId: "container-1", status: status.RUNNING, pid: -1, shimPid: -1}}, statusManager.updates)
}
