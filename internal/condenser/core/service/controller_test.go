package service

import (
	"io"
	"strings"
	"testing"

	"raind/internal/condenser/store/ssm"
	"raind/internal/condenser/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordedCommandFactory struct {
	commands []string
}

func (f *recordedCommandFactory) Command(name string, args ...string) utils.CommandExecutor {
	f.commands = append(f.commands, name+" "+strings.Join(args, " "))
	return &noopCommandExecutor{}
}

type noopCommandExecutor struct{}

func (n *noopCommandExecutor) Start() error                   { return nil }
func (n *noopCommandExecutor) Wait() error                    { return nil }
func (n *noopCommandExecutor) Run() error                     { return nil }
func (n *noopCommandExecutor) Output() ([]byte, error)        { return nil, nil }
func (n *noopCommandExecutor) CombineOutput() ([]byte, error) { return nil, nil }
func (n *noopCommandExecutor) Pid() int                       { return -1 }
func (n *noopCommandExecutor) SetEnv(envv []string)           {}
func (n *noopCommandExecutor) SetStdout(w io.Writer)          {}
func (n *noopCommandExecutor) SetStderr(w io.Writer)          {}
func (n *noopCommandExecutor) SetStdin(r io.Reader)           {}

func TestAddClusterIPJumpRuleUsesClusterIPDestination(t *testing.T) {
	commands := &recordedCommandFactory{}
	controller := &ServiceController{commandFactory: commands}

	require.NoError(t, controller.addClusterIPJumpRule("RAIND-SVC-abc-80", "10.166.255.1", "tcp", 80))

	require.Len(t, commands.commands, 2)
	assert.Contains(t, commands.commands[0], "-A PREROUTING -d 10.166.255.1/32 -p tcp --dport 80 -j RAIND-SVC-abc-80")
	assert.Contains(t, commands.commands[1], "-A OUTPUT -d 10.166.255.1/32 -p tcp --dport 80 -j RAIND-SVC-abc-80")
}

func TestServiceTypeDefaultsToClusterIP(t *testing.T) {
	assert.Equal(t, ssm.ServiceTypeClusterIP, serviceType(ssm.ServiceInfo{}))
	assert.Equal(t, ssm.ServiceTypeClusterIP, serviceType(ssm.ServiceInfo{Type: "ClusterIP"}))
	assert.Equal(t, ssm.ServiceTypeNodePort, serviceType((ssm.ServiceInfo{Type: "NodePort"})))
}
