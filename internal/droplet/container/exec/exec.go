package exec

import (
	"fmt"
	"os"
	"slices"
	"strconv"

	"raind/internal/droplet/container/audit"
	"raind/internal/droplet/container/statusflow"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
)

type Option struct {
	ContainerId string
	Tty         bool
	Entrypoint  []string
}

type Controller struct {
	CommandFactory         utils.CommandFactory
	ContainerStatusManager status.ContainerStatusManager
	SyscallHandler         utils.KernelSyscallHandler
	LoadSpec               func(containerId string) (spec.Spec, error)
	ResolveEntrypoint      func(containerPid string, containerSpec spec.Spec, entrypoint []string) ([]string, error)
	BuildNsenterCommand    func(containerPid string, containerSpec spec.Spec) []string
	ExecuteShim            func(containerPid int, opt Option) error
}

func (c *Controller) Exec(opt Option) (err error) {
	auditLog := audit.New(opt.ContainerId, "exec")
	auditLog.SetCommand(&opt.Entrypoint)
	defer auditLog.Record(&err)

	auditLog.Stage("get_status")
	containerStatus, err := statusflow.Current(c.ContainerStatusManager, opt.ContainerId)
	if err != nil {
		return err
	}

	auditLog.Stage("check_status")
	if err := statusflow.EnsureRunning(opt.ContainerId, containerStatus); err != nil {
		return err
	}

	auditLog.Stage("get_pid")
	containerPid, err := c.ContainerStatusManager.GetPidFromId(opt.ContainerId)
	if err != nil {
		return err
	}
	auditLog.SetPid(containerPid)

	auditLog.Stage("load_spec")
	containerSpec, err := c.LoadSpec(opt.ContainerId)
	if err != nil {
		return err
	}

	if opt.Tty {
		auditLog.Stage("exec_shim")
		return c.ExecuteShim(containerPid, opt)
	}

	auditLog.Stage("exec_nsenter")
	return c.executeNsenter(containerPid, containerSpec, opt)
}

func (c *Controller) executeNsenter(containerPid int, containerSpec spec.Spec, opt Option) error {
	entrypoint, err := c.ResolveEntrypoint(strconv.Itoa(containerPid), containerSpec, opt.Entrypoint)
	if err != nil {
		return err
	}

	nsenterCommand := c.BuildNsenterCommand(strconv.Itoa(containerPid), containerSpec)
	commandStr := slices.Concat(nsenterCommand, entrypoint)
	cmd := c.CommandFactory.Command(commandStr[0], commandStr[1:]...)
	cmd.SetEnv(containerSpec.Process.Env)

	logPath := utils.ContainerLogPath(opt.ContainerId)
	f, err := c.SyscallHandler.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return err
	}
	defer f.Close()
	cmd.SetStdout(f)
	cmd.SetStderr(f)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec command failed: %w: output=%q", err, readSmallLog(logPath))
	}

	return nil
}

func readSmallLog(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	const max = 4096
	if len(b) > max {
		b = b[len(b)-max:]
	}
	return string(b)
}
