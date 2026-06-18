package service

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"raind/internal/condenser/store/psm"
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

func TestReconcileCleansPreviousServiceRulesByServiceID(t *testing.T) {
	dir := t.TempDir()
	ssmHandler := ssm.NewSsmManager(ssm.NewSsmStore(filepath.Join(dir, "ssm.json")))
	psmHandler := psm.NewPsmManager(psm.NewPsmStore(filepath.Join(dir, "psm.json")))
	commands := &recordedCommandFactory{}

	oldSvc := ssm.ServiceInfo{
		ServiceId: "svc-1",
		Name:      "web",
		Namespace: "default",
		Type:      ssm.ServiceTypeNodePort,
		Ports: []ssm.ServicePort{{
			Port:       80,
			TargetPort: 80,
			Protocol:   "tcp",
		}},
	}
	newSvc := ssm.ServiceInfo{
		Name:      "web",
		Namespace: "default",
		Type:      ssm.ServiceTypeNodePort,
		Ports: []ssm.ServicePort{{
			Port:       8080,
			TargetPort: 80,
			Protocol:   "tcp",
		}},
	}
	require.NoError(t, ssmHandler.StoreService("svc-1", newSvc))

	controller := &ServiceController{
		psmHandler:     psmHandler,
		ssmHandler:     ssmHandler,
		commandFactory: commands,
		lastState: map[string]string{
			"svc-1": "old-state",
		},
		lastServices: map[string]ssm.ServiceInfo{
			"svc-1": oldSvc,
		},
	}

	require.NoError(t, controller.reconcileOnce())

	assert.Contains(t, commands.commands, "iptables -t nat -D PREROUTING -p tcp --dport 80 -j RAIND-SVC-svc-1-80")
	assert.Contains(t, commands.commands, "iptables -t nat -D OUTPUT -m addrtype --dst-type LOCAL -p tcp --dport 80 -j RAIND-SVC-svc-1-80")
	assert.Contains(t, commands.commands, "iptables -t nat -F RAIND-SVC-svc-1-80")
	assert.Contains(t, commands.commands, "iptables -t nat -X RAIND-SVC-svc-1-80")
}

func TestServiceTypeDefaultsToClusterIP(t *testing.T) {
	assert.Equal(t, ssm.ServiceTypeClusterIP, serviceType(ssm.ServiceInfo{}))
	assert.Equal(t, ssm.ServiceTypeClusterIP, serviceType(ssm.ServiceInfo{Type: "ClusterIP"}))
	assert.Equal(t, ssm.ServiceTypeNodePort, serviceType((ssm.ServiceInfo{Type: "NodePort"})))
}
