package delete

import (
	"fmt"

	"raind/internal/droplet/container/signals"
	"raind/internal/droplet/hook"
	"raind/internal/droplet/logs"
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
	var (
		specFile spec.Spec
		event    = "delete"
		stage    string
		pid      int
	)

	defer func() {
		result := "success"
		if err != nil {
			result = "fail"
		}
		_ = logs.RecordAuditLog(logs.AuditRecord{
			ContainerId: opt.ContainerId,
			Event:       event,
			Stage:       stage,
			Pid:         pid,
			Spec:        &specFile,
			Result:      result,
			Error:       err,
		})
	}()

	stage = "get_status"
	containerStatus, err := c.ContainerStatusManager.GetStatusFromId(opt.ContainerId)
	if err != nil {
		return err
	}

	stage = "check_status"
	if containerStatus != status.STOPPED && !opt.Force {
		return fmt.Errorf("container: %s is not stopped. current status: %s", opt.ContainerId, containerStatus)
	}

	stage = "kill_process_before_remove"
	if containerStatus == status.CREATED || (containerStatus == status.RUNNING && opt.Force) {
		err = c.KillInitProcess(opt.ContainerId)
		if err != nil {
			return fmt.Errorf("kill init process failed: %w", err)
		}
	}

	stage = "load_spec"
	specFile, err = c.LoadSpec(opt.ContainerId)
	if err != nil {
		return err
	}

	stage = "hook_poststop"
	err = c.ContainerHookController.RunPoststopHooks(
		opt.ContainerId,
		specFile.Hooks.Poststop,
	)
	if err != nil {
		return err
	}

	stage = "remove_state"
	err = c.ContainerStatusManager.RemoveStatusFile(opt.ContainerId)
	if err != nil {
		return err
	}

	stage = "remove_fifo"
	if containerStatus == status.CREATED {
		err = c.RemoveFifo(utils.FifoPath(opt.ContainerId))
		if err != nil {
			return err
		}
	}

	stage = "remove_cgroup"
	err = c.SyscallHandler.RemoveAll(utils.CgroupPath(opt.ContainerId))
	if err != nil {
		return err
	}

	stage = "remove_runtime_dir"
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

	return c.ContainerStatusManager.UpdateStatus(
		containerId,
		status.STOPPED,
		0,
		0,
	)
}
