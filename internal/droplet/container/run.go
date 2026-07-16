package container

import (
	"raind/internal/droplet/container/run"
	"raind/internal/droplet/hook"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
	"syscall"
)

// NewContainerRun constructs a ContainerRun using the default
// implementations of its dependencies.
func NewContainerRun() *ContainerRun {
	return &ContainerRun{
		specLoader:               newFileSpecLoader(),
		fifoCreator:              newContainerFifoHandler(),
		commandFactory:           utils.NewCommandFactory(),
		containerStart:           NewContainerStart(),
		containerCgroupPreparer:  newContainerCgroupController(),
		containerNetworkPreparer: newContainerNetworkController(),
		containerStatusManager:   status.NewStatusHandler(),
		containerHookController:  hook.NewHookController(),
	}
}

// ContainerRun is the compatibility facade for the run operation.
type ContainerRun struct {
	specLoader               specLoader
	fifoCreator              fifoCreator
	commandFactory           utils.CommandFactory
	containerStart           *ContainerStart
	containerCgroupPreparer  containerCgroupPreparer
	containerNetworkPreparer containerNetworkPreparer
	containerStatusManager   status.ContainerStatusManager
	containerHookController  hook.ContainerHookController
}

func (c *ContainerRun) Run(opt RunOption) error {
	controller := run.Controller{
		LoadSpec:               c.specLoader.loadFile,
		PrepareBundleConfig:    prepareBundleConfig,
		WriteSpecHashFile:      writeSpecHashFile,
		BundlePathForContainer: bundlePathForContainer,
		CreateFifo:             c.fifoCreator.createFifo,
		CommandFactory:         c.commandFactory,
		BuildSysProcAttr: func(containerSpec spec.Spec) *syscall.SysProcAttr {
			return buildSysProcAttr(buildProcAttrForContainer(containerSpec))
		},
		WriteContainerPidFile:   writeContainerPidFile,
		PrepareCgroup:           c.containerCgroupPreparer.prepare,
		PrepareNetwork:          c.containerNetworkPreparer.prepare,
		Start:                   runStarter{start: c.containerStart},
		ContainerStatusManager:  c.containerStatusManager,
		ContainerHookController: c.containerHookController,
	}
	return controller.Run(run.Option{
		ContainerId:  opt.ContainerId,
		Bundle:       opt.Bundle,
		PidFile:      opt.PidFile,
		Tty:          opt.Tty,
		PrintPidFlag: opt.PrintPidFlag,
	})
}

type runStarter struct {
	start *ContainerStart
}

func (s runStarter) Execute(containerId string) error {
	return s.start.Execute(StartOption{ContainerId: containerId})
}
