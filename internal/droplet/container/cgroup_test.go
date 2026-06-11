package container

import (
	"errors"
	"os"
	"path/filepath"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cgroupWriteCall struct {
	name string
	data string
	perm os.FileMode
}

type fakeCgroupSyscallHandler struct {
	utils.KernelSyscallHandler

	writeCalls []cgroupWriteCall
	writeErr   error
}

func (f *fakeCgroupSyscallHandler) WriteFile(name string, data []byte, perm os.FileMode) error {
	f.writeCalls = append(f.writeCalls, cgroupWriteCall{
		name: name,
		data: string(data),
		perm: perm,
	})
	return f.writeErr
}

func TestContainerCgroupControllerPrepareWritesResourceLimitsAndProcess(t *testing.T) {
	// == setup ==
	syscalls := &fakeCgroupSyscallHandler{}
	controller := &containerCgroupController{
		syscallHandler: syscalls,
	}
	containerId := "container-1"
	containerSpec := spec.Spec{
		LinuxSpec: spec.LinuxSpecObject{
			Resources: spec.ResourceObject{
				Memory: spec.MemoryObject{Limit: 268435456},
				Cpu:    spec.CpuObject{Quota: 50000, Period: 100000},
			},
		},
	}

	// == exercise ==
	err := controller.prepare(containerId, containerSpec, 4242)

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, []cgroupWriteCall{
		{
			name: filepath.Join(utils.CgroupPath(containerId), "memory.max"),
			data: "268435456\n",
			perm: 0644,
		},
		{
			name: filepath.Join(utils.CgroupPath(containerId), "cpu.max"),
			data: "50000 100000\n",
			perm: 0644,
		},
		{
			name: filepath.Join(utils.CgroupPath(containerId), "pids.max"),
			data: "512\n",
			perm: 0644,
		},
		{
			name: filepath.Join(utils.CgroupPath(containerId), "cgroup.procs"),
			data: "4242\n",
			perm: 0644,
		},
	}, syscalls.writeCalls)
}

func TestContainerCgroupControllerPrepareStopsOnFirstWriteError(t *testing.T) {
	// == setup ==
	syscalls := &fakeCgroupSyscallHandler{writeErr: errors.New("write failed")}
	controller := &containerCgroupController{
		syscallHandler: syscalls,
	}

	// == exercise ==
	err := controller.prepare("container-1", spec.Spec{}, 4242)

	// == assert ==
	require.Error(t, err)
	assert.Equal(t, "write failed", err.Error())
	assert.Len(t, syscalls.writeCalls, 1)
}
