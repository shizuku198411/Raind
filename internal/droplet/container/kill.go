package container

import (
	"raind/internal/droplet/container/kill"
	"raind/internal/droplet/hook"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
	"time"
)

// NewContainerKill constructs a ContainerKill with the default
// implementations of its dependencies (SyscallHandler, StatusManager).
// This serves as the main entry point for the `kill` workflow, which
// delivers a signal to a running container's init process.
func NewContainerKill() *ContainerKill {
	return &ContainerKill{
		specLoader:              newFileSpecLoader(),
		syscallHandler:          utils.NewSyscallHandler(),
		containerStatusManager:  status.NewStatusHandler(),
		containerHookController: hook.NewHookController(),
	}
}

// ContainerKill is the compatibility facade for the kill operation.
//
// The implementation lives in internal/droplet/container/kill so operation
// code can be managed independently from the parent container package.
type ContainerKill struct {
	specLoader              specLoader
	syscallHandler          utils.KernelSyscallHandler
	containerStatusManager  status.ContainerStatusManager
	containerHookController hook.ContainerHookController
}

func (c *ContainerKill) Kill(opt KillOption) error {
	controller := kill.Controller{
		LoadSpec:                c.specLoader.loadFile,
		SyscallHandler:          c.syscallHandler,
		ContainerStatusManager:  c.containerStatusManager,
		ContainerHookController: c.containerHookController,
	}
	return controller.Kill(kill.Option{
		ContainerId: opt.ContainerId,
		Signal:      opt.Signal,
	})
}

func (c *ContainerKill) hasExternalPidFileMarker(containerId string) bool {
	return kill.HasExternalPidFileMarker(c.syscallHandler, containerId)
}

func (c *ContainerKill) cleanupShim(containerId string) error {
	return kill.CleanupShim(c.syscallHandler, containerId)
}

func (c *ContainerKill) waitProcessExit(procIdentity ProcIdentity, timeout time.Duration) error {
	return kill.WaitProcessExit(kill.ProcIdentity(procIdentity), timeout)
}

type ProcIdentity = kill.ProcIdentity

func (c *ContainerKill) readProcStartTime(pid int) (uint64, error) {
	return kill.ReadProcStartTime(pid)
}

func (c *ContainerKill) readProcState(pid int) (string, error) {
	return kill.ReadProcState(pid)
}

func parseProcStatFields(s string) ([]string, error) {
	return kill.ParseProcStatFields(s)
}
