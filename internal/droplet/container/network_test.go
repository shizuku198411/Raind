package container

import (
	"errors"
	"io"
	"os"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeNetworkCommandFactory struct {
	commands []*fakeNetworkCommand
	failAt   int
	err      error
}

func (f *fakeNetworkCommandFactory) Command(name string, args ...string) utils.CommandExecutor {
	runNumber := len(f.commands) + 1
	cmd := &fakeNetworkCommand{
		name: name,
		args: append([]string(nil), args...),
	}
	if f.failAt == runNumber {
		cmd.runErr = f.err
	}
	f.commands = append(f.commands, cmd)
	return cmd
}

type fakeNetworkCommand struct {
	name   string
	args   []string
	runs   int
	runErr error
}

func (f *fakeNetworkCommand) Start() error                        { return nil }
func (f *fakeNetworkCommand) Wait() error                         { return nil }
func (f *fakeNetworkCommand) Run() error                          { f.runs++; return f.runErr }
func (f *fakeNetworkCommand) Pid() int                            { return 0 }
func (f *fakeNetworkCommand) ExitCode() int                       { return 0 }
func (f *fakeNetworkCommand) Sys() any                            { return nil }
func (f *fakeNetworkCommand) SetEnv([]string)                     {}
func (f *fakeNetworkCommand) SetStdout(io.Writer)                 {}
func (f *fakeNetworkCommand) SetStderr(io.Writer)                 {}
func (f *fakeNetworkCommand) SetStdin(io.Reader)                  {}
func (f *fakeNetworkCommand) SetSysProcAttr(*syscall.SysProcAttr) {}
func (f *fakeNetworkCommand) SetExtraFiles([]*os.File)            {}
func networkCommand(name string, args ...string) *fakeNetworkCommand {
	return &fakeNetworkCommand{name: name, args: args, runs: 1}
}
func assertNetworkCommand(t *testing.T, got *fakeNetworkCommand, want *fakeNetworkCommand) {
	t.Helper()
	assert.Equal(t, want.name, got.name)
	assert.Equal(t, want.args, got.args)
	assert.Equal(t, want.runs, got.runs)
}

func TestContainerNetworkControllerPrepareRunsExpectedCommands(t *testing.T) {
	// == setup ==
	factory := &fakeNetworkCommandFactory{}
	controller := &containerNetworkController{
		commandFactory: factory,
	}
	annotation := spec.AnnotationObject{
		Net: `{
			"hostInterface": "veth-host",
			"bridgeInterface": "raind0",
			"interface": {
				"name": "veth-container",
				"ipv4": {
					"address": "10.166.0.2/24",
					"gateway": "10.166.0.1"
				}
			}
		}`,
	}

	// == exercise ==
	err := controller.prepare("container-1", 4242, annotation)

	// == assert ==
	require.NoError(t, err)
	require.Len(t, factory.commands, 8)
	assertNetworkCommand(t, factory.commands[0], networkCommand("ip", "link", "add", "name", "veth-container", "type", "veth", "peer", "name", "veth-host", "netns", "4242"))
	assertNetworkCommand(t, factory.commands[1], networkCommand("ip", "link", "set", "veth-container", "master", "raind0"))
	assertNetworkCommand(t, factory.commands[2], networkCommand("ip", "link", "set", "veth-container", "up"))
	assertNetworkCommand(t, factory.commands[3], networkCommand("nsenter", "-t", "4242", "-n", "ip", "link", "set", "lo", "up"))
	assertNetworkCommand(t, factory.commands[4], networkCommand("nsenter", "-t", "4242", "-n", "ip", "link", "set", "veth-host", "name", "eth0"))
	assertNetworkCommand(t, factory.commands[5], networkCommand("nsenter", "-t", "4242", "-n", "ip", "addr", "add", "10.166.0.2/24", "dev", "eth0"))
	assertNetworkCommand(t, factory.commands[6], networkCommand("nsenter", "-t", "4242", "-n", "ip", "link", "set", "eth0", "up"))
	assertNetworkCommand(t, factory.commands[7], networkCommand("nsenter", "-t", "4242", "-n", "ip", "route", "add", "default", "via", "10.166.0.1"))
}

func TestContainerNetworkControllerPrepareSkipsWhenNetworkAnnotationIsEmpty(t *testing.T) {
	// == setup ==
	factory := &fakeNetworkCommandFactory{}
	controller := &containerNetworkController{
		commandFactory: factory,
	}
	annotation := spec.AnnotationObject{
		Net: `{"interface": {"name": "", "ipv4": {"address": ""}}}`,
	}

	// == exercise ==
	err := controller.prepare("container-1", 4242, annotation)

	// == assert ==
	require.NoError(t, err)
	assert.Empty(t, factory.commands)
}

func TestContainerNetworkControllerPrepareReturnsMalformedAnnotationError(t *testing.T) {
	// == setup ==
	factory := &fakeNetworkCommandFactory{}
	controller := &containerNetworkController{commandFactory: factory}

	// == exercise ==
	err := controller.prepare("container-1", 4242, spec.AnnotationObject{Net: `{"broken"`})

	// == assert ==
	require.Error(t, err)
	assert.Empty(t, factory.commands)
}

func TestContainerNetworkControllerPrepareStopsOnCommandFailure(t *testing.T) {
	// == setup ==
	factory := &fakeNetworkCommandFactory{failAt: 2, err: errors.New("ip failed")}
	controller := &containerNetworkController{commandFactory: factory}
	annotation := spec.AnnotationObject{
		Net: `{
			"hostInterface": "veth-host",
			"bridgeInterface": "raind0",
			"interface": {
				"name": "veth-container",
				"ipv4": {
					"address": "10.166.0.2/24",
					"gateway": "10.166.0.1"
				}
			}
		}`,
	}

	// == exercise ==
	err := controller.prepare("container-1", 4242, annotation)

	// == assert ==
	require.Error(t, err)
	assert.Equal(t, "ip failed", err.Error())
	require.Len(t, factory.commands, 2)
	assert.Equal(t, 1, factory.commands[0].runs)
	assert.Equal(t, 1, factory.commands[1].runs)
}
