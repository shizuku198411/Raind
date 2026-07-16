package cgroup

import (
	"fmt"
	"path/filepath"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"
	"strconv"
)

func NewController() *Controller {
	return &Controller{
		SyscallHandler: utils.NewSyscallHandler(),
	}
}

type Controller struct {
	SyscallHandler utils.KernelSyscallHandler
}

// prepare applies resource limits defined in the container spec
// and assigns the given process ID to the container's cgroup.
// This method configures memory, CPU, and process membership
// sequentially and returns an error if any step fails.
func (c *Controller) Prepare(containerId string, containerSpec spec.Spec, pid int) error {
	if !HasResourceConfig(containerSpec.LinuxSpec.Resources) {
		return nil
	}
	if err := c.SyscallHandler.MkdirAll(utils.CgroupPath(containerId), 0755); err != nil {
		return err
	}

	// 1. set memory limit
	if err := c.SetMemoryLimit(containerId, containerSpec.LinuxSpec.Resources.Memory); err != nil {
		return err
	}

	// 2. set cpu limit
	if err := c.SetCpuLimit(containerId, containerSpec.LinuxSpec.Resources.Cpu); err != nil {
		return err
	}

	// 3. set pids max
	if err := c.SetPidsLimit(containerId, containerSpec.LinuxSpec.Resources.Pids.Limit); err != nil {
		return err
	}

	// 3. set pid to cgroup.procs
	if err := c.SetProcess(containerId, pid); err != nil {
		return err
	}

	return nil
}

// setMemoryLimit writes the memory limit value to memory.max
// under the container's cgroup directory. The value is applied
// according to the provided MemoryObject configuration.
func (c *Controller) SetMemoryLimit(containerId string, memoryObject spec.MemoryObject) error {
	if memoryObject.Limit <= 0 {
		return nil
	}

	cgroupPath := utils.CgroupPath(containerId)
	memoryPath := filepath.Join(cgroupPath, "memory.max")
	memoryLimit := strconv.FormatInt(int64(memoryObject.Limit), 10)

	if err := c.SyscallHandler.WriteFile(memoryPath, []byte(memoryLimit+"\n"), 0644); err != nil {
		return err
	}

	return nil
}

// setCpuLimit writes CPU quota and period values to cpu.max
// under the container's cgroup directory. The quota and period
// together define the scheduler time allocation for the container.
func (c *Controller) SetCpuLimit(containerId string, cpuObject spec.CpuObject) error {
	if cpuObject.Quota <= 0 || cpuObject.Period <= 0 {
		return nil
	}

	cgroupPath := utils.CgroupPath(containerId)
	cpuPath := filepath.Join(cgroupPath, "cpu.max")
	cpuLimit := fmt.Sprintf("%d %d\n", cpuObject.Quota, cpuObject.Period)

	if err := c.SyscallHandler.WriteFile(cpuPath, []byte(cpuLimit), 0644); err != nil {
		return err
	}
	return nil
}

func (c *Controller) SetPidsLimit(containerId string, pids int) error {
	if pids <= 0 {
		return nil
	}

	cgroupPath := utils.CgroupPath(containerId)
	pidsMaxPath := filepath.Join(cgroupPath, "pids.max")
	pidsMax := fmt.Sprintf("%d\n", pids)

	if err := c.SyscallHandler.WriteFile(pidsMaxPath, []byte(pidsMax), 0644); err != nil {
		return err
	}
	return nil
}

// setProcessToCgroup assigns the given process ID to the container's
// cgroup by writing it into cgroup.procs. This ensures the process
// becomes subject to the configured resource limits.
func (c *Controller) SetProcess(containerId string, pid int) error {
	cgroupPath := utils.CgroupPath(containerId)
	cgroupProcs := filepath.Join(cgroupPath, "cgroup.procs")
	data := strconv.Itoa(pid) + "\n"

	if err := c.SyscallHandler.WriteFile(cgroupProcs, []byte(data), 0644); err != nil {
		return err
	}

	return nil
}

func HasResourceConfig(resources spec.ResourceObject) bool {
	return resources.Memory.Limit > 0 ||
		(resources.Cpu.Quota > 0 && resources.Cpu.Period > 0) ||
		resources.Pids.Limit > 0
}
