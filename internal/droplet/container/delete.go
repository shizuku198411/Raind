package container

import (
	deleteop "raind/internal/droplet/container/delete"
	"raind/internal/droplet/hook"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
)

// NewContainerDelete constructs a ContainerDelete with the default
// implementations of its dependencies (SpecLoader, StatusManager, HookController).
// This acts as the main entry point for the container deletion workflow.
func NewContainerDelete() *ContainerDelete {
	return &ContainerDelete{
		specLoader:              newFileSpecLoader(),
		fifoHandler:             newContainerFifoHandler(),
		containerStatusManager:  status.NewStatusHandler(),
		containerHookController: hook.NewHookController(),
		syscallHandler:          utils.NewSyscallHandler(),
	}
}

// ContainerDelete is the compatibility facade for the delete operation.
type ContainerDelete struct {
	specLoader  specLoader
	fifoHandler interface {
		removeFifo(path string) error
	}
	containerStatusManager  status.ContainerStatusManager
	containerHookController hook.ContainerHookController
	syscallHandler          utils.KernelSyscallHandler
}

func (c *ContainerDelete) Delete(opt DeleteOption) error {
	controller := deleteop.Controller{
		LoadSpec:                c.specLoader.loadFile,
		RemoveFifo:              c.fifoHandler.removeFifo,
		ContainerStatusManager:  c.containerStatusManager,
		ContainerHookController: c.containerHookController,
		SyscallHandler:          c.syscallHandler,
	}
	return controller.Delete(deleteop.Option{
		ContainerId: opt.ContainerId,
		Force:       opt.Force,
	})
}

func (c *ContainerDelete) killInitProcess(containerId string) error {
	controller := deleteop.Controller{
		ContainerStatusManager: c.containerStatusManager,
		SyscallHandler:         c.syscallHandler,
	}
	return controller.KillInitProcess(containerId)
}
