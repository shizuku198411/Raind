package service

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"raind/internal/condenser/core/container"
	"raind/internal/condenser/store/ipam"
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

type fakeEndpointContainerService struct {
	containersByPod map[string][]container.ContainerState
}

func (f *fakeEndpointContainerService) Create(container.ServiceCreateModel) (string, error) {
	return "", nil
}
func (f *fakeEndpointContainerService) Start(container.ServiceStartModel) (string, error) {
	return "", nil
}
func (f *fakeEndpointContainerService) Delete(container.ServiceDeleteModel) (string, error) {
	return "", nil
}
func (f *fakeEndpointContainerService) Stop(container.ServiceStopModel) (string, error) {
	return "", nil
}
func (f *fakeEndpointContainerService) Exec(container.ServiceExecModel) error { return nil }
func (f *fakeEndpointContainerService) GetContainerList() ([]container.ContainerState, error) {
	return nil, nil
}
func (f *fakeEndpointContainerService) GetContainerById(string) (container.ContainerState, error) {
	return container.ContainerState{}, nil
}
func (f *fakeEndpointContainerService) GetContainerStats(string) (container.ContainerStats, error) {
	return container.ContainerStats{}, nil
}
func (f *fakeEndpointContainerService) ListContainerStats() ([]container.ContainerStats, error) {
	return nil, nil
}
func (f *fakeEndpointContainerService) GetContainersByPodId(podId string) ([]container.ContainerState, error) {
	return f.containersByPod[podId], nil
}
func (f *fakeEndpointContainerService) GetContainerLogPath(string) (string, error) {
	return "", nil
}
func (f *fakeEndpointContainerService) GetContainerSpec(string) (map[string]any, error) {
	return nil, nil
}
func (f *fakeEndpointContainerService) InspectContainer(string) (container.ContainerInspect, error) {
	return container.ContainerInspect{}, nil
}
func (f *fakeEndpointContainerService) GetLogWithTailLines(string, int) ([]byte, error) {
	return nil, nil
}

type fakeEndpointIpam struct {
	addresses map[string]struct {
		host   string
		bridge string
		addr   string
	}
}

func (f *fakeEndpointIpam) Allocate(string, string) (string, error)     { return "", nil }
func (f *fakeEndpointIpam) Release(string) error                        { return nil }
func (f *fakeEndpointIpam) GetNetworkList() ([]ipam.NetworkList, error) { return nil, nil }
func (f *fakeEndpointIpam) StoreBridge(string) (string, string, error)  { return "", "", nil }
func (f *fakeEndpointIpam) RemoveBridge(string) error                   { return nil }
func (f *fakeEndpointIpam) GetRuntimeSubnet() (string, error)           { return "", nil }
func (f *fakeEndpointIpam) GetDefaultInterface() (string, error)        { return "", nil }
func (f *fakeEndpointIpam) GetDefaultInterfaceAddr() (string, error)    { return "", nil }
func (f *fakeEndpointIpam) GetBridgeAddr(string) (string, error)        { return "", nil }
func (f *fakeEndpointIpam) GetDnsProxyInfo() (string, string, []string, error) {
	return "", "", nil, nil
}
func (f *fakeEndpointIpam) GetContainerAddress(containerId string) (string, string, string, error) {
	info, ok := f.addresses[containerId]
	if !ok {
		return "", "", "", fmt.Errorf("container address not found: %s", containerId)
	}
	return info.host, info.bridge, info.addr, nil
}
func (f *fakeEndpointIpam) GetInfoByIp(string) (string, string, error)        { return "", "", nil }
func (f *fakeEndpointIpam) SetForwardInfo(string, int, int, string) error     { return nil }
func (f *fakeEndpointIpam) GetForwardInfo(string) ([]ipam.ForwardInfo, error) { return nil, nil }
func (f *fakeEndpointIpam) GetPoolList() ([]ipam.Pool, error)                 { return nil, nil }
func (f *fakeEndpointIpam) GetNetworkInfoById(string) (string, ipam.Allocation, error) {
	return "", ipam.Allocation{}, nil
}
func (f *fakeEndpointIpam) GetVethById(string) (string, error) { return "", nil }

func TestAddClusterIPJumpRuleUsesClusterIPDestination(t *testing.T) {
	commands := &recordedCommandFactory{}
	controller := &ServiceController{commandFactory: commands}

	require.NoError(t, controller.addClusterIPJumpRule("RAIND-SVC-abc-80", "10.166.255.1", "tcp", 80))

	require.Len(t, commands.commands, 2)
	assert.Contains(t, commands.commands[0], "-A PREROUTING -d 10.166.255.1/32 -p tcp --dport 80 -j RAIND-SVC-abc-80")
	assert.Contains(t, commands.commands[1], "-A OUTPUT -d 10.166.255.1/32 -p tcp --dport 80 -j RAIND-SVC-abc-80")
}

func TestBuildEndpointsOnlyIncludesReadyPods(t *testing.T) {
	dir := t.TempDir()
	psmHandler := psm.NewPsmManager(psm.NewPsmStore(filepath.Join(dir, "psm.json")))
	require.NoError(t, psmHandler.StorePodTemplate("tpl-1", psm.PodTemplateSpec{
		Containers: []psm.ContainerTemplateSpec{{Name: "web", Image: "nginx:latest"}},
	}))

	controller := &ServiceController{
		psmHandler: psmHandler,
		containerHandler: &fakeEndpointContainerService{
			containersByPod: map[string][]container.ContainerState{
				"pod-ready": {
					{ContainerId: "infra-ready", Name: utils.PodInfraContainerNamePrefix + "pod-ready", State: "running"},
					{ContainerId: "member-ready", Name: buildPodMemberName("web", "pod-ready"), State: "running"},
				},
				"pod-degraded": {
					{ContainerId: "infra-degraded", Name: utils.PodInfraContainerNamePrefix + "pod-degraded", State: "running"},
					{ContainerId: "member-degraded", Name: buildPodMemberName("web", "pod-degraded"), State: "running"},
				},
				"pod-stopped-by-user": {
					{ContainerId: "infra-stopped-by-user", Name: utils.PodInfraContainerNamePrefix + "pod-stopped-by-user", State: "running"},
					{ContainerId: "member-stopped-by-user", Name: buildPodMemberName("web", "pod-stopped-by-user"), State: "running"},
				},
				"pod-infra-stopped": {
					{ContainerId: "infra-stopped", Name: utils.PodInfraContainerNamePrefix + "pod-infra-stopped", State: "stopped"},
					{ContainerId: "member-infra-stopped", Name: buildPodMemberName("web", "pod-infra-stopped"), State: "running"},
				},
				"pod-member-stopped": {
					{ContainerId: "infra-member-stopped", Name: utils.PodInfraContainerNamePrefix + "pod-member-stopped", State: "running"},
					{ContainerId: "member-stopped", Name: buildPodMemberName("web", "pod-member-stopped"), State: "stopped"},
				},
				"pod-member-missing": {
					{ContainerId: "infra-member-missing", Name: utils.PodInfraContainerNamePrefix + "pod-member-missing", State: "running"},
				},
			},
		},
		ipamHandler: &fakeEndpointIpam{
			addresses: map[string]struct {
				host   string
				bridge string
				addr   string
			}{
				"infra-ready": {host: "eth0", bridge: "rbr0", addr: "10.166.0.2"},
			},
		},
	}
	svc := ssm.ServiceInfo{
		Namespace: "default",
		Selector:  map[string]string{"app": "web"},
	}
	pods := []psm.PodInfo{
		{PodId: "pod-ready", TemplateId: "tpl-1", Namespace: "default", State: "running", Labels: map[string]string{"app": "web"}},
		{PodId: "pod-degraded", TemplateId: "tpl-1", Namespace: "default", State: "degraded", Labels: map[string]string{"app": "web"}},
		{PodId: "pod-stopped-by-user", TemplateId: "tpl-1", Namespace: "default", State: "running", StoppedByUser: true, Labels: map[string]string{"app": "web"}},
		{PodId: "pod-infra-stopped", TemplateId: "tpl-1", Namespace: "default", State: "running", Labels: map[string]string{"app": "web"}},
		{PodId: "pod-member-stopped", TemplateId: "tpl-1", Namespace: "default", State: "running", Labels: map[string]string{"app": "web"}},
		{PodId: "pod-member-missing", TemplateId: "tpl-1", Namespace: "default", State: "running", Labels: map[string]string{"app": "web"}},
	}

	endpoints, err := controller.buildEndpoints(svc, pods)

	require.NoError(t, err)
	require.Len(t, endpoints, 1)
	assert.Equal(t, "10.166.0.2", endpoints[0].Addr)
	assert.Equal(t, "eth0", endpoints[0].HostInterface)
	assert.Equal(t, "rbr0", endpoints[0].Bridge)
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
	assert.Contains(t, commands.commands, "iptables -D FORWARD -j RAIND-FWD-svc-1")
	assert.Contains(t, commands.commands, "iptables -F RAIND-FWD-svc-1")
	assert.Contains(t, commands.commands, "iptables -X RAIND-FWD-svc-1")
}

func TestApplyRulesUsesManagedForwardChain(t *testing.T) {
	commands := &recordedCommandFactory{}
	controller := &ServiceController{commandFactory: commands}
	svc := ssm.ServiceInfo{
		ServiceId: "svc-1",
		Name:      "web",
		Namespace: "default",
		Type:      ssm.ServiceTypeNodePort,
		Ports: []ssm.ServicePort{{
			Port:       8080,
			TargetPort: 80,
			Protocol:   "tcp",
		}},
	}
	endpoints := []svcEndpoint{{
		Addr:          "10.166.0.2",
		HostInterface: "eth0",
		Bridge:        "rbr0",
	}}

	require.NoError(t, controller.applyRules(svc, endpoints))

	assert.Contains(t, commands.commands, "iptables -N RAIND-FWD-svc-1")
	assert.Contains(t, commands.commands, "iptables -F RAIND-FWD-svc-1")
	assert.Contains(t, commands.commands, "iptables -D FORWARD -j RAIND-FWD-svc-1")
	assert.Contains(t, commands.commands, "iptables -I FORWARD 1 -j RAIND-FWD-svc-1")
	assert.Contains(t, commands.commands, "iptables -A RAIND-FWD-svc-1 -i rbr0 -o rbr0 -p tcp -m conntrack --ctstate DNAT --dport 80 -d 10.166.0.2 -j ACCEPT")
	assert.Contains(t, commands.commands, "iptables -A RAIND-FWD-svc-1 -i eth0 -o rbr0 -p tcp --dport 80 -d 10.166.0.2 -j ACCEPT")
	assert.Contains(t, commands.commands, "iptables -A RAIND-FWD-svc-1 -o eth0 -i rbr0 -p tcp --sport 80 -s 10.166.0.2 -j ACCEPT")
	assert.NotContains(t, commands.commands, "iptables -A FORWARD -i eth0 -o rbr0 -p tcp --dport 80 -d 10.166.0.2 -j ACCEPT")
	assert.NotContains(t, commands.commands, "iptables -A FORWARD -o eth0 -i rbr0 -p tcp --sport 80 -s 10.166.0.2 -j ACCEPT")
}

func TestServiceTypeDefaultsToClusterIP(t *testing.T) {
	assert.Equal(t, ssm.ServiceTypeClusterIP, serviceType(ssm.ServiceInfo{}))
	assert.Equal(t, ssm.ServiceTypeClusterIP, serviceType(ssm.ServiceInfo{Type: "ClusterIP"}))
	assert.Equal(t, ssm.ServiceTypeNodePort, serviceType((ssm.ServiceInfo{Type: "NodePort"})))
}
