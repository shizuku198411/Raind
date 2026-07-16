package container

import (
	startop "raind/internal/droplet/container/start"
	"raind/internal/droplet/hook"
	"raind/internal/droplet/status"
)

// NewContainerStart returns a ContainerStart wired with the default
// FIFO handler implementation. This is the standard entry point for
// executing the container start phase.
func NewContainerStart() *ContainerStart {
	return &ContainerStart{
		specLoader:              newFileSpecLoader(),
		fifoHandler:             newContainerFifoHandler(),
		containerStatusManager:  status.NewStatusHandler(),
		containerHookController: hook.NewHookController(),
	}
}

// ContainerStart coordinates the logic for starting a container
// from the runtime side.
//
// The start phase signals the already-created init process by
// writing to the FIFO and then removes the FIFO after the signal
// is delivered.
type ContainerStart struct {
	specLoader  specLoader
	fifoHandler interface {
		writeFifo(path string) error
		removeFifo(path string) error
	}
	containerStatusManager  status.ContainerStatusManager
	containerHookController hook.ContainerHookController
}

// Execute performs the container start sequence for the given container.
//
// The sequence is:
//
//  1. Open and write to the FIFO to notify the init process that it may start
//  2. Remove the FIFO after the notification is complete
//
// An error is returned if either the write or removal operation fails.
func (c *ContainerStart) Execute(opt StartOption) (err error) {
	controller := startop.Controller{
		LoadSpec:                c.specLoader.loadFile,
		WriteFifo:               c.fifoHandler.writeFifo,
		RemoveFifo:              c.fifoHandler.removeFifo,
		ContainerStatusManager:  c.containerStatusManager,
		ContainerHookController: c.containerHookController,
	}
	return controller.Execute(startop.Option{ContainerId: opt.ContainerId})
}
