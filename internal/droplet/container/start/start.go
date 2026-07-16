package start

import (
	"raind/internal/droplet/container/audit"
	"raind/internal/droplet/container/statusflow"
	"raind/internal/droplet/hook"
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
	var specFile spec.Spec

	auditLog := audit.New(opt.ContainerId, "start")
	auditLog.SetSpec(&specFile)
	defer auditLog.Record(&err)

	auditLog.Stage("check_status")
	containerStatus, err := statusflow.Current(c.ContainerStatusManager, opt.ContainerId)
	if err != nil {
		return err
	}
	if err := statusflow.EnsureCreated(opt.ContainerId, containerStatus); err != nil {
		return err
	}

	auditLog.Stage("load_spec")
	specFile, err = c.LoadSpec(opt.ContainerId)
	if err != nil {
		return err
	}

	auditLog.Stage("hook_startContainer")
	err = c.ContainerHookController.RunStartContainerHooks(
		opt.ContainerId,
		specFile.Hooks.StartContainer,
	)
	if err != nil {
		return err
	}

	auditLog.Stage("write_fifo")
	fifo := utils.FifoPath(opt.ContainerId)
	err = c.WriteFifo(fifo)
	if err != nil {
		return err
	}

	auditLog.Stage("remove_fifo")
	err = c.RemoveFifo(fifo)
	if err != nil {
		return err
	}

	auditLog.Stage("update_state")
	err = statusflow.Transition(
		c.ContainerStatusManager,
		opt.ContainerId,
		status.RUNNING,
		-1, // no update
		-1, // no update
	)
	if err != nil {
		return err
	}

	auditLog.Stage("hook_poststart")
	err = c.ContainerHookController.RunPoststartHooks(
		opt.ContainerId,
		specFile.Hooks.Poststart,
	)
	if err != nil {
		return err
	}

	return nil
}
