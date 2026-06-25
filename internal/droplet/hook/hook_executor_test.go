package hook

import (
	"context"
	"errors"
	"io"
	"os"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeHookStatusManager struct {
	state string
	pid   int
}

func (f *fakeHookStatusManager) CreateStatusFile(string, int, status.ContainerStatus, string, string, spec.AnnotationObject) error {
	return nil
}
func (f *fakeHookStatusManager) RemoveStatusFile(string) error { return nil }
func (f *fakeHookStatusManager) ReadStatusFile(string) (string, error) {
	return f.state, nil
}
func (f *fakeHookStatusManager) UpdateStatus(string, status.ContainerStatus, int, int) error {
	return nil
}
func (f *fakeHookStatusManager) UpdateExitCode(string, int) error { return nil }
func (f *fakeHookStatusManager) UpdateReasonAndMessage(string, string, string) error {
	return nil
}
func (f *fakeHookStatusManager) GetPidFromId(string) (int, error) { return f.pid, nil }
func (f *fakeHookStatusManager) GetStatusFromId(string) (status.ContainerStatus, error) {
	return status.RUNNING, nil
}
func (f *fakeHookStatusManager) GetShimPidFromId(string) (int, error) { return 0, nil }
func (f *fakeHookStatusManager) ListContainers() ([]status.StatusObject, error) {
	return nil, nil
}

type fakeHookCommandFactory struct {
	commands []*fakeHookCommand
	failAt   int
}

func (f *fakeHookCommandFactory) Command(name string, args ...string) utils.CommandExecutor {
	cmd := &fakeHookCommand{name: name, args: append([]string(nil), args...)}
	if f.failAt == len(f.commands)+1 {
		cmd.err = errors.New("hook failed")
	}
	f.commands = append(f.commands, cmd)
	return cmd
}

type fakeContextHookCommandFactory struct {
	fakeHookCommandFactory
	contextCommands int
}

func (f *fakeContextHookCommandFactory) CommandContext(ctx context.Context, name string, args ...string) utils.CommandExecutor {
	f.contextCommands++
	cmd := f.Command(name, args...)
	if ctx.Err() != nil {
		if fake, ok := cmd.(*fakeHookCommand); ok {
			fake.err = ctx.Err()
		}
	}
	return cmd
}

type fakeHookCommand struct {
	name      string
	args      []string
	env       []string
	stdinData string
	stderr    io.Writer
	err       error
	runs      int
}

func (f *fakeHookCommand) Start() error { return nil }
func (f *fakeHookCommand) Wait() error  { return nil }
func (f *fakeHookCommand) Run() error {
	f.runs++
	if f.stderr != nil {
		_, _ = f.stderr.Write([]byte("stderr"))
	}
	return f.err
}
func (f *fakeHookCommand) Pid() int              { return 0 }
func (f *fakeHookCommand) ExitCode() int         { return 0 }
func (f *fakeHookCommand) Sys() any              { return nil }
func (f *fakeHookCommand) SetEnv(env []string)   { f.env = append([]string(nil), env...) }
func (f *fakeHookCommand) SetStdout(io.Writer)   {}
func (f *fakeHookCommand) SetStderr(w io.Writer) { f.stderr = w }
func (f *fakeHookCommand) SetStdin(r io.Reader) {
	data, _ := io.ReadAll(r)
	f.stdinData = string(data)
}
func (f *fakeHookCommand) SetSysProcAttr(*syscall.SysProcAttr) {}
func (f *fakeHookCommand) SetExtraFiles([]*os.File)            {}

func TestHookControllerEmptyHookListIsNoop(t *testing.T) {
	// == setup ==
	factory := &fakeHookCommandFactory{}
	controller := &HookController{
		commandFactory:         factory,
		containerStatusManager: &fakeHookStatusManager{state: `{"id":"container-1"}`},
	}

	// == exercise/assert ==
	require.NoError(t, controller.RunCreateRuntimeHooks("container-1", nil))
	assert.Empty(t, factory.commands)
}

func TestHookControllerRunHookListPassesCommandEnvAndStateStdin(t *testing.T) {
	// == setup ==
	factory := &fakeHookCommandFactory{}
	controller := &HookController{
		commandFactory:         factory,
		containerStatusManager: &fakeHookStatusManager{state: `{"id":"container-1"}`},
	}

	// == exercise ==
	err := controller.RunCreateRuntimeHooks("container-1", []spec.HookObject{
		{Path: "/bin/hook", Args: []string{"a", "b"}, Env: []string{"A=1"}},
	})

	// == assert ==
	require.NoError(t, err)
	require.Len(t, factory.commands, 1)
	cmd := factory.commands[0]
	assert.Equal(t, "/bin/hook", cmd.name)
	assert.Equal(t, []string{"b"}, cmd.args)
	assert.Equal(t, []string{"A=1"}, cmd.env)
	assert.Equal(t, `{"id":"container-1"}`, cmd.stdinData)
	assert.Equal(t, 1, cmd.runs)
}

func TestHookControllerRunHookListDropsOCIArgv0(t *testing.T) {
	// == setup ==
	factory := &fakeHookCommandFactory{}
	controller := &HookController{
		commandFactory:         factory,
		containerStatusManager: &fakeHookStatusManager{state: `{"id":"container-1"}`},
	}

	// == exercise ==
	err := controller.RunPoststartHooks("container-1", []spec.HookObject{
		{Path: "/bin/sh", Args: []string{"sh", "-c", "cat >/tmp/hook-state"}},
	})

	// == assert ==
	require.NoError(t, err)
	require.Len(t, factory.commands, 1)
	assert.Equal(t, "/bin/sh", factory.commands[0].name)
	assert.Equal(t, []string{"-c", "cat >/tmp/hook-state"}, factory.commands[0].args)
}

func TestHookControllerRunHookListKeepsLegacyFlagArgs(t *testing.T) {
	// == setup ==
	factory := &fakeHookCommandFactory{}
	controller := &HookController{
		commandFactory:         factory,
		containerStatusManager: &fakeHookStatusManager{state: `{"id":"container-1"}`},
	}

	// == exercise ==
	err := controller.RunCreateRuntimeHooks("container-1", []spec.HookObject{
		{Path: "/bin/hook", Args: []string{"--url", "https://example.test"}},
	})

	// == assert ==
	require.NoError(t, err)
	require.Len(t, factory.commands, 1)
	assert.Equal(t, []string{"--url", "https://example.test"}, factory.commands[0].args)
}

func TestHookControllerRunHookListWithNsenterBuildsNsenterCommand(t *testing.T) {
	// == setup ==
	factory := &fakeHookCommandFactory{}
	controller := &HookController{
		commandFactory:         factory,
		containerStatusManager: &fakeHookStatusManager{state: `{"id":"container-1"}`, pid: 4242},
	}

	// == exercise ==
	err := controller.RunStartContainerHooks("container-1", []spec.HookObject{
		{Path: "/bin/hook", Args: []string{"a"}},
	})

	// == assert ==
	require.NoError(t, err)
	require.Len(t, factory.commands, 1)
	cmd := factory.commands[0]
	assert.Equal(t, "/usr/bin/nsenter", cmd.name)
	assert.Equal(t, []string{"-t", "4242", "-m", "-u", "-i", "-n", "-p", "--", "/bin/hook"}, cmd.args)
	assert.Equal(t, `{"id":"container-1"}`, cmd.stdinData)
}

func TestHookControllerRunHookListStopsOnFailure(t *testing.T) {
	// == setup ==
	factory := &fakeHookCommandFactory{failAt: 1}
	controller := &HookController{
		commandFactory:         factory,
		containerStatusManager: &fakeHookStatusManager{state: `{"id":"container-1"}`},
	}

	// == exercise ==
	err := controller.RunPoststopHooks("container-1", []spec.HookObject{
		{Path: "/bin/fail"},
		{Path: "/bin/skip"},
	})

	// == assert ==
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hook poststop[0] failed")
	require.Len(t, factory.commands, 1)
}

func TestHookControllerRunHookListRejectsEmptyPath(t *testing.T) {
	// == setup ==
	controller := &HookController{
		commandFactory:         &fakeHookCommandFactory{},
		containerStatusManager: &fakeHookStatusManager{state: `{"id":"container-1"}`},
	}

	// == exercise ==
	err := controller.RunPoststartHooks("container-1", []spec.HookObject{{}})

	// == assert ==
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty path")
}

func TestHookControllerRunHookListUsesTimeoutContextWhenConfigured(t *testing.T) {
	// == setup ==
	timeout := 1
	factory := &fakeContextHookCommandFactory{}
	controller := &HookController{
		commandFactory:         factory,
		containerStatusManager: &fakeHookStatusManager{state: `{"id":"container-1"}`},
	}

	// == exercise ==
	err := controller.RunPoststartHooks("container-1", []spec.HookObject{{
		Path:    "/bin/hook",
		Timeout: &timeout,
	}})

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, 1, factory.contextCommands)
	require.Len(t, factory.commands, 1)
	assert.Equal(t, "/bin/hook", factory.commands[0].name)
}
