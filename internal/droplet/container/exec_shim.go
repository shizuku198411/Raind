package container

import (
	shimpkg "raind/internal/droplet/container/shim"
	"raind/internal/droplet/utils"
)

func NewContainerExecShim() *ContainerExecShim {
	return &ContainerExecShim{
		specLoader:     newFileSpecLoader(),
		commandFactory: &utils.ExecCommandFactory{},
	}
}

type ContainerExecShim struct {
	specLoader     specLoader
	commandFactory utils.CommandFactory
}

func (c *ContainerExecShim) Execute(containerId string, containerPid string, entrypoint []string) error {
	execShim := shimpkg.ExecShim{
		LoadSpec:            c.specLoader.loadFile,
		CommandFactory:      c.commandFactory,
		ResolveEntrypoint:   resolveExecEntrypoint,
		BuildNsenterCommand: buildExecNsenterCommand,
	}
	return execShim.Execute(containerId, containerPid, entrypoint)
}
