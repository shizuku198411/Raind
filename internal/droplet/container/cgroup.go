package container

import (
	cgrouppkg "raind/internal/droplet/container/cgroup"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"
)

func newContainerCgroupController() *containerCgroupController {
	syscallHandler := utils.NewSyscallHandler()
	return &containerCgroupController{
		syscallHandler: syscallHandler,
		controller:     &cgrouppkg.Controller{SyscallHandler: syscallHandler},
	}
}

type containerCgroupPreparer interface {
	prepare(containerId string, spec spec.Spec, pid int) error
}

type containerCgroupController struct {
	syscallHandler utils.KernelSyscallHandler
	controller     *cgrouppkg.Controller
}

func (c *containerCgroupController) cgroupController() *cgrouppkg.Controller {
	if c.controller == nil || c.controller.SyscallHandler != c.syscallHandler {
		c.controller = &cgrouppkg.Controller{SyscallHandler: c.syscallHandler}
	}
	return c.controller
}

func (c *containerCgroupController) prepare(containerId string, containerSpec spec.Spec, pid int) error {
	return c.cgroupController().Prepare(containerId, containerSpec, pid)
}

func (c *containerCgroupController) setMemoryLimit(containerId string, memoryObject spec.MemoryObject) error {
	return c.cgroupController().SetMemoryLimit(containerId, memoryObject)
}

func (c *containerCgroupController) setCpuLimit(containerId string, cpuObject spec.CpuObject) error {
	return c.cgroupController().SetCpuLimit(containerId, cpuObject)
}

func (c *containerCgroupController) setPidsLimit(containerId string, pids int) error {
	return c.cgroupController().SetPidsLimit(containerId, pids)
}

func (c *containerCgroupController) setProcessToCgroup(containerId string, pid int) error {
	return c.cgroupController().SetProcess(containerId, pid)
}

func hasCgroupResourceConfig(resources spec.ResourceObject) bool {
	return cgrouppkg.HasResourceConfig(resources)
}

func (c *containerCgroupController) setSyscallHandler(syscallHandler utils.KernelSyscallHandler) {
	c.syscallHandler = syscallHandler
	c.controller = &cgrouppkg.Controller{SyscallHandler: syscallHandler}
}
