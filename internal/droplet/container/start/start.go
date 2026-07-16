package start

import (
	"fmt"

	"raind/internal/droplet/hook"
	"raind/internal/droplet/logs"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
)

type Option struct {
	ContainerId string
}

type LoadSpecFunc func(containerId string) (spec.Spec, error)
type WriteFifoFunc func(path string) error
type RemoveFifoFunc func(path string) error

type Controller struct {
	LoadSpec                LoadSpecFunc
	WriteFifo               WriteFifoFunc
	RemoveFifo              RemoveFifoFunc
	ContainerStatusManager  status.ContainerStatusManager
	ContainerHookController hook.ContainerHookController
}

func (c *Controller) Execute(opt Option) (err error) {
	var (
		specFile spec.Spec
		event    = "start"
		stage    string
	)

	defer func() {
		result := "success"
		if err != nil {
			result = "fail"
		}
		_ = logs.RecordAuditLog(logs.AuditRecord{
			ContainerId: opt.ContainerId,
			Event:       event,
			Spec:        &specFile,
			Stage:       stage,
			Result:      result,
			Error:       err,
		})
	}()

	stage = "check_status"
	containerStatus, err := c.ContainerStatusManager.GetStatusFromId(opt.ContainerId)
	if err != nil {
		return err
	}
	if containerStatus != status.CREATED {
		return fmt.Errorf("container: %s is not created. currnet status: %s", opt.ContainerId, containerStatus)
	}

	stage = "load_spec"
	specFile, err = c.LoadSpec(opt.ContainerId)
	if err != nil {
		return err
	}

	stage = "hook_startContainer"
	err = c.ContainerHookController.RunStartContainerHooks(
		opt.ContainerId,
		specFile.Hooks.StartContainer,
	)
	if err != nil {
		return err
	}

	stage = "write_fifo"
	fifo := utils.FifoPath(opt.ContainerId)
	err = c.WriteFifo(fifo)
	if err != nil {
		return err
	}

	stage = "remove_fifo"
	err = c.RemoveFifo(fifo)
	if err != nil {
		return err
	}

	stage = "update_state"
	err = c.ContainerStatusManager.UpdateStatus(
		opt.ContainerId,
		status.RUNNING,
		-1, // no update
		-1, // no update
	)
	if err != nil {
		return err
	}

	stage = "hook_poststart"
	err = c.ContainerHookController.RunPoststartHooks(
		opt.ContainerId,
		specFile.Hooks.Poststart,
	)
	if err != nil {
		return err
	}

	return nil
}
