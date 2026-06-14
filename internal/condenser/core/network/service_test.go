package network

import (
	"errors"
	"io"
	"testing"

	"raind/internal/condenser/core/policy"
	"raind/internal/condenser/store/ipam"
	"raind/internal/condenser/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNetworkServiceCreateBridgeStoresIPAMThenCreatesBridgeAndDnsRedirect(t *testing.T) {
	ipamHandler := &fakeNetworkIpamHandler{dnsProxyAddr: "10.166.254.254"}
	commands := &fakeNetworkCommandFactory{
		runErrors: []error{
			errors.New("not found"),
			nil,
			nil,
			nil,
			errors.New("not found"),
			nil,
			errors.New("not found"),
			nil,
		},
	}
	policyHandler := &fakePolicyService{}
	service := &NetworkService{commandFactory: commands, ipamHandler: ipamHandler, policyHandler: policyHandler}

	err := service.CreateNewNetwork(ServiceNewNetworkModel{Bridge: "raind1"})

	require.NoError(t, err)
	assert.Equal(t, []string{"raind1"}, ipamHandler.storedBridges)
	assert.Equal(t, []networkCommandCall{
		{name: "ip", args: []string{"-o", "link", "show"}},
		{name: "ip", args: []string{"link", "show", "raind1"}},
		{name: "ip", args: []string{"link", "add", "raind1", "type", "bridge"}},
		{name: "ip", args: []string{"addr", "add", "10.166.1.254/24", "dev", "raind1"}},
		{name: "ip", args: []string{"link", "set", "raind1", "up"}},
		{name: "iptables", args: []string{"-t", "nat", "-C", "PREROUTING", "-i", "raind1", "-p", "udp", "--dport", "53", "-j", "DNAT", "--to-destination", "10.166.254.254:1053"}},
		{name: "iptables", args: []string{"-t", "nat", "-A", "PREROUTING", "-i", "raind1", "-p", "udp", "--dport", "53", "-j", "DNAT", "--to-destination", "10.166.254.254:1053"}},
		{name: "iptables", args: []string{"-t", "nat", "-C", "PREROUTING", "-i", "raind1", "-p", "tcp", "--dport", "53", "-j", "DNAT", "--to-destination", "10.166.254.254:1053"}},
		{name: "iptables", args: []string{"-t", "nat", "-A", "PREROUTING", "-i", "raind1", "-p", "tcp", "--dport", "53", "-j", "DNAT", "--to-destination", "10.166.254.254:1053"}},
	}, commands.calls)
	assert.Equal(t, 1, policyHandler.commitCalls)
}

func TestNetworkServiceRemoveNetworkDeletesDnsRedirectRules(t *testing.T) {
	ipamHandler := &fakeNetworkIpamHandler{
		dnsProxyAddr: "10.166.254.254",
		networkList: []ipam.NetworkList{
			{Interface: "raind1", NumContainers: 0},
		},
	}
	commands := &fakeNetworkCommandFactory{}
	policyHandler := &fakePolicyService{}
	service := &NetworkService{commandFactory: commands, ipamHandler: ipamHandler, policyHandler: policyHandler}

	err := service.RemoveNetwork(ServiceRemoveNetworkModel{Bridge: "raind1"})

	require.NoError(t, err)
	assert.Equal(t, []string{"raind1"}, ipamHandler.removedBridges)
	assert.Equal(t, []networkCommandCall{
		{name: "iptables", args: []string{"-t", "nat", "-C", "PREROUTING", "-i", "raind1", "-p", "udp", "--dport", "53", "-j", "DNAT", "--to-destination", "10.166.254.254:1053"}},
		{name: "iptables", args: []string{"-t", "nat", "-D", "PREROUTING", "-i", "raind1", "-p", "udp", "--dport", "53", "-j", "DNAT", "--to-destination", "10.166.254.254:1053"}},
		{name: "iptables", args: []string{"-t", "nat", "-C", "PREROUTING", "-i", "raind1", "-p", "tcp", "--dport", "53", "-j", "DNAT", "--to-destination", "10.166.254.254:1053"}},
		{name: "iptables", args: []string{"-t", "nat", "-D", "PREROUTING", "-i", "raind1", "-p", "tcp", "--dport", "53", "-j", "DNAT", "--to-destination", "10.166.254.254:1053"}},
		{name: "ip", args: []string{"link", "show", "raind1"}},
		{name: "ip", args: []string{"link", "del", "raind1"}},
	}, commands.calls)
	assert.Equal(t, 1, policyHandler.commitCalls)
}

func TestNetworkServiceRemoveNetworkIgnoresMissingDnsRedirectRules(t *testing.T) {
	ipamHandler := &fakeNetworkIpamHandler{
		dnsProxyAddr: "10.166.254.254",
		networkList: []ipam.NetworkList{
			{Interface: "raind1", NumContainers: 0},
		},
	}
	commands := &fakeNetworkCommandFactory{
		runErrors: []error{
			errors.New("udp rule not found"),
			errors.New("tcp rule not found"),
			nil,
			nil,
		},
	}
	policyHandler := &fakePolicyService{}
	service := &NetworkService{commandFactory: commands, ipamHandler: ipamHandler, policyHandler: policyHandler}

	err := service.RemoveNetwork(ServiceRemoveNetworkModel{Bridge: "raind1"})

	require.NoError(t, err)
	assert.Equal(t, []networkCommandCall{
		{name: "iptables", args: []string{"-t", "nat", "-C", "PREROUTING", "-i", "raind1", "-p", "udp", "--dport", "53", "-j", "DNAT", "--to-destination", "10.166.254.254:1053"}},
		{name: "iptables", args: []string{"-t", "nat", "-C", "PREROUTING", "-i", "raind1", "-p", "tcp", "--dport", "53", "-j", "DNAT", "--to-destination", "10.166.254.254:1053"}},
		{name: "ip", args: []string{"link", "show", "raind1"}},
		{name: "ip", args: []string{"link", "del", "raind1"}},
	}, commands.calls)
	assert.Equal(t, []string{"raind1"}, ipamHandler.removedBridges)
	assert.Equal(t, 1, policyHandler.commitCalls)
}

func TestNetworkServiceCreateBridgeRollsBackIPAMWhenBridgeCreationFails(t *testing.T) {
	ipamHandler := &fakeNetworkIpamHandler{}
	commands := &fakeNetworkCommandFactory{
		runErrors: []error{errors.New("not found"), errors.New("ip link add failed")},
	}
	policyHandler := &fakePolicyService{}
	service := &NetworkService{commandFactory: commands, ipamHandler: ipamHandler, policyHandler: policyHandler}

	err := service.CreateNewNetwork(ServiceNewNetworkModel{Bridge: "raind1"})

	require.Error(t, err)
	assert.Equal(t, []string{"raind1"}, ipamHandler.removedBridges)
	assert.Equal(t, 1, policyHandler.commitCalls)
}

func TestNetworkServiceCreateBridgeRejectsUnmanagedNamespaceBridge(t *testing.T) {
	ipamHandler := &fakeNetworkIpamHandler{
		networkList: []ipam.NetworkList{
			{Interface: "raind0"},
			{Interface: "rns-managed"},
		},
	}
	commands := &fakeNetworkCommandFactory{
		outputs: [][]byte{[]byte("1: lo: <LOOPBACK>\n2: rns-managed: <BROADCAST>\n3: rns-stale@if10: <BROADCAST>\n")},
	}
	service := &NetworkService{commandFactory: commands, ipamHandler: ipamHandler, policyHandler: &fakePolicyService{}}

	err := service.CreateNewNetwork(ServiceNewNetworkModel{Bridge: "raind1"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmanaged namespace bridge exists: rns-stale")
	assert.Empty(t, ipamHandler.storedBridges)
	assert.Equal(t, []networkCommandCall{
		{name: "ip", args: []string{"-o", "link", "show"}},
	}, commands.calls)
}

func TestParseLinkNames(t *testing.T) {
	out := "1: lo: <LOOPBACK>\n2: rns-demo@if10: <BROADCAST>\n3: rd_abc@if2: <BROADCAST>\n"

	assert.Equal(t, []string{"lo", "rns-demo", "rd_abc"}, parseLinkNames(out))
}

type networkCommandCall struct {
	name string
	args []string
}

type fakeNetworkCommandFactory struct {
	calls      []networkCommandCall
	runErrors  []error
	outputs    [][]byte
	outputErrs []error
}

func (f *fakeNetworkCommandFactory) Command(name string, args ...string) utils.CommandExecutor {
	f.calls = append(f.calls, networkCommandCall{name: name, args: append([]string{}, args...)})
	var output []byte
	if len(f.outputs) > 0 {
		output = f.outputs[0]
		f.outputs = f.outputs[1:]
	}
	var outputErr error
	if len(f.outputErrs) > 0 {
		outputErr = f.outputErrs[0]
		f.outputErrs = f.outputErrs[1:]
	}
	return &fakeNetworkCommandExecutor{factory: f, output: output, outputErr: outputErr}
}

type fakeNetworkCommandExecutor struct {
	factory   *fakeNetworkCommandFactory
	output    []byte
	outputErr error
}

func (e *fakeNetworkCommandExecutor) popRunErr() error {
	if e.factory == nil || len(e.factory.runErrors) == 0 {
		return nil
	}
	err := e.factory.runErrors[0]
	e.factory.runErrors = e.factory.runErrors[1:]
	return err
}

func (e *fakeNetworkCommandExecutor) Start() error                   { return e.popRunErr() }
func (e *fakeNetworkCommandExecutor) Wait() error                    { return nil }
func (e *fakeNetworkCommandExecutor) Run() error                     { return e.popRunErr() }
func (e *fakeNetworkCommandExecutor) Output() ([]byte, error)        { return e.output, e.outputErr }
func (e *fakeNetworkCommandExecutor) CombineOutput() ([]byte, error) { return e.output, e.outputErr }
func (e *fakeNetworkCommandExecutor) Pid() int                       { return 123 }
func (e *fakeNetworkCommandExecutor) SetEnv([]string)                {}
func (e *fakeNetworkCommandExecutor) SetStdout(io.Writer)            {}
func (e *fakeNetworkCommandExecutor) SetStderr(io.Writer)            {}
func (e *fakeNetworkCommandExecutor) SetStdin(io.Reader)             {}

type fakeNetworkIpamHandler struct {
	storedBridges  []string
	removedBridges []string
	networkList    []ipam.NetworkList
	dnsProxyAddr   string
}

func (f *fakeNetworkIpamHandler) Allocate(string, string) (string, error) { return "", nil }
func (f *fakeNetworkIpamHandler) Release(string) error                    { return nil }
func (f *fakeNetworkIpamHandler) GetNetworkList() ([]ipam.NetworkList, error) {
	return f.networkList, nil
}
func (f *fakeNetworkIpamHandler) StoreBridge(bridge string) (string, string, error) {
	f.storedBridges = append(f.storedBridges, bridge)
	return "10.166.1.0/24", "10.166.1.254/24", nil
}
func (f *fakeNetworkIpamHandler) RemoveBridge(bridge string) error {
	f.removedBridges = append(f.removedBridges, bridge)
	return nil
}
func (f *fakeNetworkIpamHandler) GetRuntimeSubnet() (string, error) { return "", nil }
func (f *fakeNetworkIpamHandler) GetDefaultInterface() (string, error) {
	return "", nil
}
func (f *fakeNetworkIpamHandler) GetDefaultInterfaceAddr() (string, error) {
	return "", nil
}
func (f *fakeNetworkIpamHandler) GetBridgeAddr(string) (string, error) {
	return "", nil
}
func (f *fakeNetworkIpamHandler) GetDnsProxyInfo() (string, string, []string, error) {
	addr := f.dnsProxyAddr
	if addr == "" {
		addr = "10.166.254.254"
	}
	return "raindDns", addr, []string{"8.8.8.8"}, nil
}
func (f *fakeNetworkIpamHandler) GetContainerAddress(string) (string, string, string, error) {
	return "", "", "", nil
}
func (f *fakeNetworkIpamHandler) GetInfoByIp(string) (string, string, error) {
	return "", "", nil
}
func (f *fakeNetworkIpamHandler) SetForwardInfo(string, int, int, string) error { return nil }
func (f *fakeNetworkIpamHandler) GetForwardInfo(string) ([]ipam.ForwardInfo, error) {
	return nil, nil
}
func (f *fakeNetworkIpamHandler) GetPoolList() ([]ipam.Pool, error) { return nil, nil }
func (f *fakeNetworkIpamHandler) GetNetworkInfoById(string) (string, ipam.Allocation, error) {
	return "", ipam.Allocation{}, nil
}
func (f *fakeNetworkIpamHandler) GetVethById(string) (string, error) { return "", nil }

type fakePolicyService struct {
	commitCalls int
}

func (f *fakePolicyService) BuildPredefinedPolicy() error { return nil }
func (f *fakePolicyService) BuildUserPolicy() error       { return nil }
func (f *fakePolicyService) GetPolicyList(policy.ServiceListModel) policy.PolicyListModel {
	return policy.PolicyListModel{}
}
func (f *fakePolicyService) ChangeNSMode(string) error { return nil }
func (f *fakePolicyService) AddUserPolicy(policy.ServiceAddPolicyModel) (string, error) {
	return "", nil
}
func (f *fakePolicyService) RemoveUserPolicy(policy.ServiceRemovePolicyModel) error { return nil }
func (f *fakePolicyService) CommitPolicy() error {
	f.commitCalls++
	return nil
}
func (f *fakePolicyService) RevertPolicy() error { return nil }
