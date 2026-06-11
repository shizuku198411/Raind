package container

import (
	"os"
	"raind/internal/droplet/utils"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAppArmorSyscallHandler struct {
	utils.KernelSyscallHandler

	enabled bool
	opens   []string
	files   []*os.File
}

func (f *fakeAppArmorSyscallHandler) Stat(name string) (os.FileInfo, error) {
	if name == "/sys/module/apparmor/parameters/enabled" && f.enabled {
		tmp, err := os.CreateTemp("", "raind-aa-enabled-*")
		if err != nil {
			return nil, err
		}
		defer tmp.Close()
		return tmp.Stat()
	}
	return nil, os.ErrNotExist
}

func (f *fakeAppArmorSyscallHandler) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	f.opens = append(f.opens, name)
	tmp, err := os.CreateTemp("", "raind-aa-write-*")
	if err != nil {
		return nil, err
	}
	f.files = append(f.files, tmp)
	return tmp, nil
}

func TestAppArmorApplyProfileNoopsWhenDisabled(t *testing.T) {
	// == setup ==
	syscalls := &fakeAppArmorSyscallHandler{}
	manager := &AppArmorManager{syscallHandler: syscalls}

	// == exercise ==
	err := manager.ApplyAAProfile("raind-default")

	// == assert ==
	require.NoError(t, err)
	assert.Empty(t, syscalls.opens)
}

func TestAppArmorApplyProfileWritesChangeProfile(t *testing.T) {
	// == setup ==
	syscalls := &fakeAppArmorSyscallHandler{enabled: true}
	manager := &AppArmorManager{syscallHandler: syscalls}

	// == exercise ==
	err := manager.ApplyAAProfile(" raind-default ")

	// == assert ==
	require.NoError(t, err)
	require.Equal(t, []string{"/proc/self/attr/current"}, syscalls.opens)
	require.Len(t, syscalls.files, 1)
	_, err = syscalls.files[0].Seek(0, 0)
	require.NoError(t, err)
	data, err := os.ReadFile(syscalls.files[0].Name())
	require.NoError(t, err)
	assert.Equal(t, "changeprofile raind-default", string(data))
}

func TestAppArmorApplyProfileOnExecWritesExecProfile(t *testing.T) {
	// == setup ==
	syscalls := &fakeAppArmorSyscallHandler{enabled: true}
	manager := &AppArmorManager{syscallHandler: syscalls}

	// == exercise ==
	err := manager.ApplyAAProfileOnExec("raind-default")

	// == assert ==
	require.NoError(t, err)
	require.Equal(t, []string{"/proc/self/attr/exec"}, syscalls.opens)
	require.Len(t, syscalls.files, 1)
	data, err := os.ReadFile(syscalls.files[0].Name())
	require.NoError(t, err)
	assert.Equal(t, "exec raind-default", string(data))
}
