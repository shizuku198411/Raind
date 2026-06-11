package container

import (
	"io"
	"os"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeExecCommandFactory struct {
	commands []*fakeExecCommand
}

func (f *fakeExecCommandFactory) Command(name string, args ...string) utils.CommandExecutor {
	cmd := &fakeExecCommand{
		name: name,
		args: append([]string(nil), args...),
	}
	f.commands = append(f.commands, cmd)
	return cmd
}

type fakeExecCommand struct {
	name string
	args []string

	starts int

	stdout io.Writer
	stderr io.Writer
	stdin  io.Reader
	attr   *syscall.SysProcAttr
}

func (f *fakeExecCommand) Start() error                          { f.starts++; return nil }
func (f *fakeExecCommand) Wait() error                           { return nil }
func (f *fakeExecCommand) Run() error                            { return nil }
func (f *fakeExecCommand) Pid() int                              { return 0 }
func (f *fakeExecCommand) ExitCode() int                         { return 0 }
func (f *fakeExecCommand) Sys() any                              { return nil }
func (f *fakeExecCommand) SetEnv([]string)                       {}
func (f *fakeExecCommand) SetStdout(w io.Writer)                 { f.stdout = w }
func (f *fakeExecCommand) SetStderr(w io.Writer)                 { f.stderr = w }
func (f *fakeExecCommand) SetStdin(r io.Reader)                  { f.stdin = r }
func (f *fakeExecCommand) SetSysProcAttr(a *syscall.SysProcAttr) { f.attr = a }
func (f *fakeExecCommand) SetExtraFiles([]*os.File)              {}

func TestContainerExecNonTTYStartsNsenterAndRedirectsLogs(t *testing.T) {
	// == setup ==
	commandFactory := &fakeExecCommandFactory{}
	statusManager := &fakeDeleteStatusManager{status: status.RUNNING, pid: 1234}
	syscalls := &fakeDeleteSyscallHandler{}
	execController := &ContainerExec{
		commandFactory:         commandFactory,
		containerStatusManager: statusManager,
		syscallHandler:         syscalls,
	}

	// == exercise ==
	err := execController.Exec(ExecOption{
		ContainerId: "container-1",
		Entrypoint:  []string{"/bin/echo", "hello"},
	})

	// == assert ==
	require.NoError(t, err)
	require.Len(t, commandFactory.commands, 1)
	cmd := commandFactory.commands[0]
	assert.Equal(t, "nsenter", cmd.name)
	assert.Equal(t, []string{"-t", "1234", "--all", "/bin/echo", "hello"}, cmd.args)
	assert.Equal(t, 1, cmd.starts)
	assert.NotNil(t, cmd.stdout)
	assert.NotNil(t, cmd.stderr)
	assert.Equal(t, []string{utils.ExecLogPath("container-1")}, syscalls.openFiles)
}

func TestContainerExecTTYStartsExecShim(t *testing.T) {
	// == setup ==
	commandFactory := &fakeExecCommandFactory{}
	statusManager := &fakeDeleteStatusManager{status: status.RUNNING, pid: 1234}
	execController := &ContainerExec{
		commandFactory:         commandFactory,
		containerStatusManager: statusManager,
		syscallHandler:         &fakeDeleteSyscallHandler{},
	}

	// == exercise ==
	err := execController.Exec(ExecOption{
		ContainerId: "container-1",
		Tty:         true,
		Entrypoint:  []string{"/bin/sh"},
	})

	// == assert ==
	require.NoError(t, err)
	require.Len(t, commandFactory.commands, 1)
	cmd := commandFactory.commands[0]
	assert.Equal(t, os.Args[0], cmd.name)
	assert.Equal(t, []string{"exec-shim", "container-1", "1234", "/bin/sh"}, cmd.args)
	assert.Equal(t, 1, cmd.starts)
}

func TestContainerExecRejectsNonRunningContainer(t *testing.T) {
	// == setup ==
	commandFactory := &fakeExecCommandFactory{}
	execController := &ContainerExec{
		commandFactory:         commandFactory,
		containerStatusManager: &fakeDeleteStatusManager{status: status.STOPPED},
		syscallHandler:         &fakeDeleteSyscallHandler{},
	}

	// == exercise ==
	err := execController.Exec(ExecOption{ContainerId: "container-1", Entrypoint: []string{"/bin/sh"}})

	// == assert ==
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
	assert.Empty(t, commandFactory.commands)
}
