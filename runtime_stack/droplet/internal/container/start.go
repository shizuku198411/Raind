package container

import (
	"droplet/internal/hook"
	"droplet/internal/logs"
	"droplet/internal/spec"
	"droplet/internal/status"
	"droplet/internal/utils"
	"fmt"
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
	var (
		spec  spec.Spec
		event = "start"
		stage string
	)

	// audit log
	defer func() {
		result := "success"
		if err != nil {
			result = "fail"
		}
		_ = logs.RecordAuditLog(logs.AuditRecord{
			ContainerId: opt.ContainerId,
			Event:       event,
			Spec:        &spec,
			Stage:       stage,
			Result:      result,
			Error:       err,
		})
	}()

	// 1. check container status
	//    if status is running, return error
	stage = "check_status"
	containerStatus, err := c.containerStatusManager.GetStatusFromId(opt.ContainerId)
	if err != nil {
		return err
	}
	if containerStatus != status.CREATED {
		return fmt.Errorf("container: %s is not created. currnet status: %s", opt.ContainerId, containerStatus)
	}

	// 2. load config.json
	stage = "load_spec"
	spec, err = c.specLoader.loadFile(opt.ContainerId)
	if err != nil {
		return err
	}

	// 3. HOOK: startContainer
	stage = "hook_startContainer"
	err = c.containerHookController.RunStartContainerHooks(
		opt.ContainerId,
		spec.Hooks.StartContainer,
	)
	if err != nil {
		return err
	}

	// 4. write fifo
	stage = "write_fifo"
	fifo := utils.FifoPath(opt.ContainerId)
	err = c.fifoHandler.writeFifo(fifo)
	if err != nil {
		return err
	}

	// 5. remove fifo
	stage = "remove_fifo"
	err = c.fifoHandler.removeFifo(fifo)
	if err != nil {
		return err
	}

	// 6. update status file
	//      status = running
	stage = "update_state"
	err = c.containerStatusManager.UpdateStatus(
		opt.ContainerId,
		status.RUNNING,
		-1, // no update
		-1, // no update
	)
	if err != nil {
		return err
	}

	// 7. HOOK: poststart
	stage = "hook_poststart"
	err = c.containerHookController.RunPoststartHooks(
		opt.ContainerId,
		spec.Hooks.Poststart,
	)
	if err != nil {
		return err
	}

	return nil
}
