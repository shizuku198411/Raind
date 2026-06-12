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

func TestNetworkServiceCreateBridgeStoresIPAMThenCreatesBridge(t *testing.T) {
	ipamHandler := &fakeNetworkIpamHandler{}
	commands := &fakeNetworkCommandFactory{
		runErrors: []error{errors.New("not found"), nil, nil, nil},
	}
	policyHandler := &fakePolicyService{}
	service := &NetworkService{commandFactory: commands, ipamHandler: ipamHandler, policyHandler: policyHandler}

	err := service.CreateNewNetwork(ServiceNewNetworkModel{Bridge: "raind1"})

	require.NoError(t, err)
	assert.Equal(t, []string{"raind1"}, ipamHandler.storedBridges)
	assert.Equal(t, []networkCommandCall{
		{name: "ip", args: []string{"link", "show", "raind1"}},
		{name: "ip", args: []string{"link", "add", "raind1", "type", "bridge"}},
		{name: "ip", args: []string{"addr", "add", "10.166.1.254/24", "dev", "raind1"}},
		{name: "ip", args: []string{"link", "set", "raind1", "up"}},
	}, commands.calls)
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

type networkCommandCall struct {
	name string
	args []string
}

type fakeNetworkCommandFactory struct {
	calls     []networkCommandCall
	runErrors []error
}

func (f *fakeNetworkCommandFactory) Command(name string, args ...string) utils.CommandExecutor {
	f.calls = append(f.calls, networkCommandCall{name: name, args: append([]string{}, args...)})
	var err error
	if len(f.runErrors) > 0 {
		err = f.runErrors[0]
		f.runErrors = f.runErrors[1:]
	}
	return &fakeNetworkCommandExecutor{err: err}
}

type fakeNetworkCommandExecutor struct {
	err error
}

func (e *fakeNetworkCommandExecutor) Start() error                   { return e.err }
func (e *fakeNetworkCommandExecutor) Wait() error                    { return e.err }
func (e *fakeNetworkCommandExecutor) Run() error                     { return e.err }
func (e *fakeNetworkCommandExecutor) Output() ([]byte, error)        { return nil, e.err }
func (e *fakeNetworkCommandExecutor) CombineOutput() ([]byte, error) { return nil, e.err }
func (e *fakeNetworkCommandExecutor) Pid() int                       { return 123 }
func (e *fakeNetworkCommandExecutor) SetEnv([]string)                {}
func (e *fakeNetworkCommandExecutor) SetStdout(io.Writer)            {}
func (e *fakeNetworkCommandExecutor) SetStderr(io.Writer)            {}
func (e *fakeNetworkCommandExecutor) SetStdin(io.Reader)             {}

type fakeNetworkIpamHandler struct {
	storedBridges  []string
	removedBridges []string
}

func (f *fakeNetworkIpamHandler) Allocate(string, string) (string, error) { return "", nil }
func (f *fakeNetworkIpamHandler) Release(string) error                    { return nil }
func (f *fakeNetworkIpamHandler) GetNetworkList() ([]ipam.NetworkList, error) {
	return nil, nil
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
	return "", "", nil, nil
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
