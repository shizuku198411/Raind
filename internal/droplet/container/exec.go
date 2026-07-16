package container

import (
	execop "raind/internal/droplet/container/exec"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
	"strconv"
)

func NewContainerExec() *ContainerExec {
	return &ContainerExec{
		commandFactory:         utils.NewCommandFactory(),
		containerStatusManager: status.NewStatusHandler(),
		syscallHandler:         utils.NewSyscallHandler(),
		specLoader:             newFileSpecLoader(),
	}
}

type ContainerExec struct {
	commandFactory         utils.CommandFactory
	containerStatusManager status.ContainerStatusManager
	syscallHandler         utils.KernelSyscallHandler
	specLoader             specLoader
}

func (c *ContainerExec) Exec(opt ExecOption) error {
	controller := execop.Controller{
		CommandFactory:         c.commandFactory,
		ContainerStatusManager: c.containerStatusManager,
		SyscallHandler:         c.syscallHandler,
		LoadSpec:               c.specLoader.loadFile,
		ResolveEntrypoint:      resolveExecEntrypoint,
		BuildNsenterCommand:    buildExecNsenterCommand,
		ExecuteShim: func(containerPid int, execOpt execop.Option) error {
			return c.executeShim(containerPid, ExecOption{
				ContainerId: execOpt.ContainerId,
				Tty:         execOpt.Tty,
				Entrypoint:  execOpt.Entrypoint,
			})
		},
	}
	return controller.Exec(execop.Option{
		ContainerId: opt.ContainerId,
		Tty:         opt.Tty,
		Entrypoint:  opt.Entrypoint,
	})
}

func (c *ContainerExec) executeShim(containerPid int, opt ExecOption) error {
	entrypoint := opt.Entrypoint
	shimArgs := append([]string{"exec-shim", opt.ContainerId, strconv.Itoa(containerPid)}, entrypoint...)
	cmd := c.commandFactory.Command(utils.SelfBinPath(), shimArgs...)
	return cmd.Start()
}
