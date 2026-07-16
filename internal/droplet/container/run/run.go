package run

import (
	"fmt"
	"os"
	"syscall"

	"raind/internal/droplet/hook"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
)

type Option struct {
	ContainerId  string
	Bundle       string
	PidFile      string
	Tty          bool
	PrintPidFlag bool
}

type Starter interface {
	Execute(containerId string) error
}

type Controller struct {
	LoadSpec                func(containerId string) (spec.Spec, error)
	PrepareBundleConfig     func(containerId string, bundle string) error
	WriteSpecHashFile       func(containerId string) error
	BundlePathForContainer  func(containerId string, bundle string) (string, error)
	CreateFifo              func(path string) error
	CommandFactory          utils.CommandFactory
	BuildSysProcAttr        func(containerSpec spec.Spec) *syscall.SysProcAttr
	WriteContainerPidFile   func(pidFile string, pid int) error
	PrepareCgroup           func(containerId string, containerSpec spec.Spec, pid int) error
	PrepareNetwork          func(containerId string, pid int, annotation spec.AnnotationObject) error
	Start                   Starter
	ContainerStatusManager  status.ContainerStatusManager
	ContainerHookController hook.ContainerHookController
}

func (c *Controller) Run(opt Option) error {
	if err := c.PrepareBundleConfig(opt.ContainerId, opt.Bundle); err != nil {
		return err
	}
	containerSpec, err := c.LoadSpec(opt.ContainerId)
	if err != nil {
		return err
	}
	if err := c.WriteSpecHashFile(opt.ContainerId); err != nil {
		return err
	}
	bundlePath, err := c.BundlePathForContainer(opt.ContainerId, opt.Bundle)
	if err != nil {
		return err
	}

	if err := c.ContainerStatusManager.CreateStatusFile(
		opt.ContainerId,
		0,
		status.CREATING,
		containerSpec.Root.Path,
		bundlePath,
		containerSpec.Annotations,
	); err != nil {
		return err
	}

	if err := c.ContainerHookController.RunCreateRuntimeHooks(
		opt.ContainerId,
		containerSpec.Hooks.CreateRuntime,
	); err != nil {
		return err
	}

	fifo := utils.FifoPath(opt.ContainerId)
	if err := c.CreateFifo(fifo); err != nil {
		return err
	}

	entrypoint := containerSpec.Process.Args
	initArgs := append([]string{"init", opt.ContainerId, fifo}, entrypoint...)
	cmd := c.CommandFactory.Command(utils.SelfBinPath(), initArgs...)
	tty := opt.Tty || containerSpec.Process.Terminal
	if tty {
		cmd.SetStdout(os.Stdout)
		cmd.SetStderr(os.Stderr)
		cmd.SetStdin(os.Stdin)
	}
	cmd.SetSysProcAttr(c.BuildSysProcAttr(containerSpec))

	if err := cmd.Start(); err != nil {
		return err
	}
	initPid := cmd.Pid()
	if err := c.WriteContainerPidFile(opt.PidFile, initPid); err != nil {
		return err
	}

	if opt.PrintPidFlag {
		fmt.Printf("create container success. pid: %d\n", initPid)
	} else {
		fmt.Printf("create container success. ID: %s\n", opt.ContainerId)
	}

	if err := c.PrepareCgroup(opt.ContainerId, containerSpec, initPid); err != nil {
		return err
	}

	if err := c.PrepareNetwork(opt.ContainerId, initPid, containerSpec.Annotations); err != nil {
		return err
	}

	if err := c.ContainerStatusManager.UpdateStatus(
		opt.ContainerId,
		status.CREATED,
		initPid,
		0,
	); err != nil {
		return err
	}
	if len(containerSpec.Hooks.Prestart) > 0 {
		if err := c.ContainerHookController.RunCreateRuntimeHooks(
			opt.ContainerId,
			containerSpec.Hooks.Prestart,
		); err != nil {
			return err
		}
	}

	if err := c.ContainerHookController.RunCreateContainerHooks(
		opt.ContainerId,
		containerSpec.Hooks.CreateContainer,
	); err != nil {
		return err
	}

	if err := c.ContainerHookController.RunStartContainerHooks(
		opt.ContainerId,
		containerSpec.Hooks.StartContainer,
	); err != nil {
		return err
	}

	if err := c.Start.Execute(opt.ContainerId); err != nil {
		return err
	}

	if err := c.ContainerStatusManager.UpdateStatus(
		opt.ContainerId,
		status.RUNNING,
		-1,
		-1,
	); err != nil {
		return err
	}

	if err := c.ContainerHookController.RunPoststartHooks(
		opt.ContainerId,
		containerSpec.Hooks.Poststart,
	); err != nil {
		return err
	}

	if tty {
		if err := cmd.Wait(); err != nil {
			return err
		}

		if err := c.ContainerStatusManager.UpdateStatus(
			opt.ContainerId,
			status.STOPPED,
			0,
			0,
		); err != nil {
			return err
		}
	}

	return nil
}
