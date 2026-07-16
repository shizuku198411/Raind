package delete

import (
	"fmt"

	"raind/internal/droplet/container/audit"
	"raind/internal/droplet/container/signals"
	"raind/internal/droplet/container/statusflow"
	"raind/internal/droplet/hook"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
)

type Option struct {
	ContainerId string
	Force       bool
}

type Controller struct {
	LoadSpec                func(containerId string) (spec.Spec, error)
	RemoveFifo              func(path string) error
	ContainerStatusManager  status.ContainerStatusManager
	ContainerHookController hook.ContainerHookController
	SyscallHandler          utils.KernelSyscallHandler
}

func (c *Controller) Delete(opt Option) (err error) {
	var specFile spec.Spec

	auditLog := audit.New(opt.ContainerId, "delete")
	auditLog.SetSpec(&specFile)
	defer auditLog.Record(&err)

	auditLog.Stage("get_status")
	containerStatus, err := statusflow.Current(c.ContainerStatusManager, opt.ContainerId)
	if err != nil {
		return err
	}

	auditLog.Stage("check_status")
	if !statusflow.CanDelete(containerStatus, opt.Force) {
		return fmt.Errorf("container: %s is not stopped. current status: %s", opt.ContainerId, containerStatus)
	}

	auditLog.Stage("kill_process_before_remove")
	if statusflow.ShouldKillBeforeDelete(containerStatus, opt.Force) {
		err = c.KillInitProcess(opt.ContainerId)
		if err != nil {
			return fmt.Errorf("kill init process failed: %w", err)
		}
	}

	auditLog.Stage("load_spec")
	specFile, err = c.LoadSpec(opt.ContainerId)
	if err != nil {
		return err
	}

	auditLog.Stage("hook_poststop")
	err = c.ContainerHookController.RunPoststopHooks(
		opt.ContainerId,
		specFile.Hooks.Poststop,
	)
	if err != nil {
		return err
	}

	auditLog.Stage("remove_state")
	err = statusflow.Remove(c.ContainerStatusManager, opt.ContainerId)
	if err != nil {
		return err
	}

	auditLog.Stage("remove_fifo")
	if containerStatus == status.CREATED {
		err = c.RemoveFifo(utils.FifoPath(opt.ContainerId))
		if err != nil {
			return err
		}
	}

	auditLog.Stage("remove_cgroup")
	err = c.SyscallHandler.RemoveAll(utils.CgroupPath(opt.ContainerId))
	if err != nil {
		return err
	}

	auditLog.Stage("remove_runtime_dir")
	err = c.SyscallHandler.RemoveAll(utils.ContainerDir(opt.ContainerId))
	if err != nil {
		return err
	}

	return nil
}

func (c *Controller) KillInitProcess(containerId string) error {
	containerPid, containerPidErr := c.ContainerStatusManager.GetPidFromId(containerId)
	if containerPidErr != nil {
		return containerPidErr
	}

	if err := c.SyscallHandler.Kill(containerPid, signals.Map["KILL"]); err != nil {
		return err
	}

	return statusflow.Transition(
		c.ContainerStatusManager,
		containerId,
		status.STOPPED,
		0,
		0,
	)
}
