package container

import (
	"errors"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"raind/internal/condenser/core/image"
	"raind/internal/condenser/core/network"
	"raind/internal/condenser/runtime"
	"raind/internal/condenser/store/csm"
	"raind/internal/condenser/store/ilm"
	"raind/internal/condenser/store/ipam"
	"raind/internal/condenser/store/psm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerServiceCreatePullsMissingImageAndStoresContainer(t *testing.T) {
	deps := newContainerServiceTestDeps(false)

	id, err := deps.service.Create(ServiceCreateModel{Image: "alpine:3.20", Name: "web"})

	require.NoError(t, err)
	assert.NotEmpty(t, id)
	assert.Equal(t, 1, deps.image.pullCalls)
	assert.True(t, deps.runtime.specCalled)
	assert.True(t, deps.runtime.createCalled)
	require.Contains(t, deps.csm.containers, id)
	assert.Equal(t, "web", deps.csm.containers[id].ContainerName)
	assert.Equal(t, "creating", deps.csm.containers[id].State)
}

func TestContainerServiceCreateSkipsPullWhenImageExists(t *testing.T) {
	deps := newContainerServiceTestDeps(true)

	_, err := deps.service.Create(ServiceCreateModel{Image: "alpine:3.20", Name: "web"})

	require.NoError(t, err)
	assert.Zero(t, deps.image.pullCalls)
}

func TestContainerServiceCreateRejectsDuplicateName(t *testing.T) {
	deps := newContainerServiceTestDeps(true)
	deps.csm.nameToID["web"] = "existing"

	_, err := deps.service.Create(ServiceCreateModel{Image: "alpine:3.20", Name: "web"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "already used")
	assert.False(t, deps.runtime.specCalled)
}

func TestContainerServiceCreateRollbackRemovesCSMEntryOnSpecFailure(t *testing.T) {
	deps := newContainerServiceTestDeps(true)
	deps.runtime.specErr = errors.New("spec failed")

	_, err := deps.service.Create(ServiceCreateModel{Image: "alpine:3.20", Name: "web"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "create spec failed")
	assert.Empty(t, deps.csm.containers)
	assert.Len(t, deps.ipam.released, 1)
}

func TestContainerServiceCreateRollbackReleasesAddressOnRuntimeCreateFailure(t *testing.T) {
	deps := newContainerServiceTestDeps(true)
	deps.runtime.createErr = errors.New("create failed")

	_, err := deps.service.Create(ServiceCreateModel{Image: "alpine:3.20", Name: "web"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "create container failed")
	assert.Empty(t, deps.csm.containers)
	assert.Len(t, deps.ipam.released, 1)
}

func TestContainerServiceStartResolvesNameToID(t *testing.T) {
	deps := newContainerServiceTestDeps(true)
	deps.csm.storeInfo("cid-1", csm.ContainerInfo{ContainerId: "cid-1", ContainerName: "web", State: "created"})

	id, err := deps.service.Start(ServiceStartModel{ContainerId: "web"})

	require.NoError(t, err)
	assert.Equal(t, "cid-1", id)
	assert.Equal(t, "cid-1", deps.runtime.startedID)
}

func TestContainerServiceStopRejectsNonRunningContainer(t *testing.T) {
	deps := newContainerServiceTestDeps(true)
	deps.csm.storeInfo("cid-1", csm.ContainerInfo{ContainerId: "cid-1", ContainerName: "web", State: "created"})

	_, err := deps.service.Stop(ServiceStopModel{ContainerId: "web"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "stop operation not allowed")
}

func TestContainerServiceDeleteRemovesCSMEntryAfterRuntimeDelete(t *testing.T) {
	deps := newContainerServiceTestDeps(true)
	deps.csm.storeInfo("cid-1", csm.ContainerInfo{ContainerId: "cid-1", ContainerName: "web", State: "created"})

	id, err := deps.service.Delete(ServiceDeleteModel{ContainerId: "web"})

	require.NoError(t, err)
	assert.Equal(t, "cid-1", id)
	assert.True(t, deps.runtime.deleteCalled)
	assert.Empty(t, deps.csm.containers)
}

func TestContainerServiceDeleteIgnoresCSMEntryAlreadyRemovedByPoststopHook(t *testing.T) {
	deps := newContainerServiceTestDeps(true)
	deps.csm.storeInfo("cid-1", csm.ContainerInfo{ContainerId: "cid-1", ContainerName: "web", State: "created"})
	deps.runtime.deleteHook = func(containerId string) {
		_ = deps.csm.RemoveContainer(containerId)
	}

	id, err := deps.service.Delete(ServiceDeleteModel{ContainerId: "web"})

	require.NoError(t, err)
	assert.Equal(t, "cid-1", id)
	assert.True(t, deps.runtime.deleteCalled)
	assert.Empty(t, deps.csm.containers)
}

func TestContainerServiceExecResolvesNameAndPassesCommand(t *testing.T) {
	deps := newContainerServiceTestDeps(true)
	deps.csm.storeInfo("cid-1", csm.ContainerInfo{ContainerId: "cid-1", ContainerName: "web", State: "running"})

	err := deps.service.Exec(ServiceExecModel{ContainerId: "web", Tty: true, Entrypoint: []string{"/bin/echo", "ok"}})

	require.NoError(t, err)
	assert.Equal(t, runtime.ExecModel{ContainerId: "cid-1", Tty: true, Entrypoint: []string{"/bin/echo", "ok"}}, deps.runtime.execModel)
}

type containerServiceTestDeps struct {
	service *ContainerService
	csm     *fakeCsmHandler
	ilm     *fakeIlmHandler
	ipam    *fakeIpamHandler
	image   *fakeImageService
	runtime *fakeRuntimeHandler
}

func newContainerServiceTestDeps(imageExists bool) containerServiceTestDeps {
	csmHandler := newFakeCsmHandler()
	ilmHandler := &fakeIlmHandler{exists: imageExists}
	ipamHandler := &fakeIpamHandler{}
	imageHandler := &fakeImageService{}
	runtimeHandler := &fakeRuntimeHandler{}
	service := &ContainerService{
		filesystemHandler:     &fakeFilesystemHandler{},
		runtimeHandler:        runtimeHandler,
		ipamHandler:           ipamHandler,
		ilmHandler:            ilmHandler,
		csmHandler:            csmHandler,
		psmHandler:            &fakePsmHandler{},
		imageServiceHandler:   imageHandler,
		networkServiceHandler: &fakeNetworkService{},
	}
	return containerServiceTestDeps{
		service: service,
		csm:     csmHandler,
		ilm:     ilmHandler,
		ipam:    ipamHandler,
		image:   imageHandler,
		runtime: runtimeHandler,
	}
}

type fakeRuntimeHandler struct {
	specCalled   bool
	createCalled bool
	deleteCalled bool
	startedID    string
	execModel    runtime.ExecModel
	specErr      error
	createErr    error
	deleteHook   func(containerId string)
}

func (f *fakeRuntimeHandler) Spec(runtime.SpecModel) error {
	f.specCalled = true
	return f.specErr
}
func (f *fakeRuntimeHandler) Create(runtime.CreateModel, int) error {
	f.createCalled = true
	return f.createErr
}
func (f *fakeRuntimeHandler) Start(m runtime.StartModel) error {
	f.startedID = m.ContainerId
	return nil
}
func (f *fakeRuntimeHandler) Delete(m runtime.DeleteModel) error {
	f.deleteCalled = true
	if f.deleteHook != nil {
		f.deleteHook(m.ContainerId)
	}
	return nil
}
func (f *fakeRuntimeHandler) Stop(runtime.StopModel) error { return nil }
func (f *fakeRuntimeHandler) Exec(m runtime.ExecModel) error {
	f.execModel = m
	return nil
}

type fakeImageService struct {
	pullCalls int
}

func (f *fakeImageService) Pull(image.ServicePullModel) error {
	f.pullCalls++
	return nil
}
func (f *fakeImageService) Remove(image.ServiceRemoveModel) error { return nil }
func (f *fakeImageService) Build(image.ServiceBuildModel) (string, error) {
	return "", nil
}
func (f *fakeImageService) GetImageConfig(string) (image.ImageConfigFile, error) {
	return image.ImageConfigFile{Config: image.ImageConfigObject{Entrypoint: []string{"/bin/sh"}, Cmd: []string{"-c", "echo ok"}, WorkingDir: "/"}}, nil
}
func (f *fakeImageService) GetImageList() ([]image.ImageInfo, error) { return nil, nil }
func (f *fakeImageService) GetImageStatus(string) (image.ImageStatusInfo, error) {
	return image.ImageStatusInfo{}, nil
}
func (f *fakeImageService) GetImageFsInfo(string) (image.ImageFsInfo, error) {
	return image.ImageFsInfo{}, nil
}

type fakeIlmHandler struct {
	exists bool
}

func (f *fakeIlmHandler) StoreImage(string, string, string, string, string) error { return nil }
func (f *fakeIlmHandler) RemoveImage(string, string) error                        { return nil }
func (f *fakeIlmHandler) GetBundlePath(string, string) (string, error)            { return "/bundle", nil }
func (f *fakeIlmHandler) GetConfigPath(string, string) (string, error)            { return "/config.json", nil }
func (f *fakeIlmHandler) GetRootfsPath(string, string) (string, error)            { return "/rootfs", nil }
func (f *fakeIlmHandler) GetImageInfo(string, string) (ilm.ImageInfo, error) {
	return ilm.ImageInfo{}, nil
}
func (f *fakeIlmHandler) GetImageList() ([]ilm.ImageInfo, error) { return nil, nil }
func (f *fakeIlmHandler) IsImageExist(string, string) bool       { return f.exists }

type fakeCsmHandler struct {
	containers map[string]csm.ContainerInfo
	nameToID   map[string]string
}

func newFakeCsmHandler() *fakeCsmHandler {
	return &fakeCsmHandler{containers: map[string]csm.ContainerInfo{}, nameToID: map[string]string{}}
}

func (f *fakeCsmHandler) storeInfo(id string, info csm.ContainerInfo) {
	f.containers[id] = info
	if info.ContainerName != "" {
		f.nameToID[info.ContainerName] = id
	}
}

func (f *fakeCsmHandler) StoreContainer(containerId string, state string, pid int, tty bool, repo, ref string, command []string, name string, bottleId string, logPath string, podId string) error {
	f.storeInfo(containerId, csm.ContainerInfo{
		ContainerId:   containerId,
		ContainerName: name,
		PodId:         podId,
		State:         state,
		Pid:           pid,
		Tty:           tty,
		Repository:    repo,
		Reference:     ref,
		Command:       command,
		BottleId:      bottleId,
		LogPath:       logPath,
		CreatedAt:     time.Now(),
	})
	return nil
}
func (f *fakeCsmHandler) RemoveContainer(containerId string) error {
	info, ok := f.containers[containerId]
	if !ok {
		return fmt.Errorf("containerId=%s not found", containerId)
	}
	delete(f.nameToID, info.ContainerName)
	delete(f.containers, containerId)
	return nil
}
func (f *fakeCsmHandler) UpdateContainer(containerId string, state string, pid int) error {
	info, ok := f.containers[containerId]
	if !ok {
		return fmt.Errorf("containerId=%s not found", containerId)
	}
	info.State = state
	info.Pid = pid
	f.containers[containerId] = info
	return nil
}
func (f *fakeCsmHandler) UpdateExitStatus(string, int, string, string) error { return nil }
func (f *fakeCsmHandler) UpdateSpiffe(string, string) error                  { return nil }
func (f *fakeCsmHandler) GetContainerList() ([]csm.ContainerInfo, error) {
	var out []csm.ContainerInfo
	for _, info := range f.containers {
		out = append(out, info)
	}
	return out, nil
}
func (f *fakeCsmHandler) GetContainerById(containerId string) (csm.ContainerInfo, error) {
	info, ok := f.containers[containerId]
	if !ok {
		return csm.ContainerInfo{}, fmt.Errorf("container: %s not found", containerId)
	}
	return info, nil
}
func (f *fakeCsmHandler) GetContainersByPodId(string) ([]csm.ContainerInfo, error) { return nil, nil }
func (f *fakeCsmHandler) IsNameAlreadyUsed(name string) bool {
	_, ok := f.nameToID[name]
	return ok
}
func (f *fakeCsmHandler) GetContainerIdByName(name string) (string, error) {
	id, ok := f.nameToID[name]
	if !ok {
		return "", fmt.Errorf("container: %s not found", name)
	}
	return id, nil
}
func (f *fakeCsmHandler) GetContainerNameById(containerId string) (string, error) {
	info, err := f.GetContainerById(containerId)
	if err != nil {
		return "", err
	}
	return info.ContainerName, nil
}
func (f *fakeCsmHandler) GetContainerIdAndName(str string) (string, string, error) {
	id, err := f.ResolveContainerId(str)
	if err != nil {
		return "", "", err
	}
	info, err := f.GetContainerById(id)
	if err != nil {
		return "", "", err
	}
	return id, info.ContainerName, nil
}
func (f *fakeCsmHandler) GetSpiffeById(string) (string, error) { return "", nil }
func (f *fakeCsmHandler) ResolveContainerId(str string) (string, error) {
	if _, ok := f.containers[str]; ok {
		return str, nil
	}
	return f.GetContainerIdByName(str)
}
func (f *fakeCsmHandler) IsContainerExist(str string) bool {
	_, ok := f.containers[str]
	if ok {
		return true
	}
	_, ok = f.nameToID[str]
	return ok
}
func (f *fakeCsmHandler) GetLogPath(string) (string, error) { return "", nil }

type fakeIpamHandler struct {
	released []string
}

func (f *fakeIpamHandler) Allocate(containerId string, bridge string) (string, error) {
	return "10.166.0.10", nil
}
func (f *fakeIpamHandler) Release(containerId string) error {
	f.released = append(f.released, containerId)
	return nil
}
func (f *fakeIpamHandler) GetNetworkList() ([]ipam.NetworkList, error) { return nil, nil }
func (f *fakeIpamHandler) StoreBridge(string) (string, string, error)  { return "", "", nil }
func (f *fakeIpamHandler) RemoveBridge(string) error                   { return nil }
func (f *fakeIpamHandler) GetRuntimeSubnet() (string, error)           { return "10.166.0.0/16", nil }
func (f *fakeIpamHandler) GetDefaultInterface() (string, error)        { return "eth0", nil }
func (f *fakeIpamHandler) GetDefaultInterfaceAddr() (string, error) {
	return "192.168.0.2/24", nil
}
func (f *fakeIpamHandler) GetBridgeAddr(string) (string, error) {
	return "10.166.0.254/24", nil
}
func (f *fakeIpamHandler) GetDnsProxyInfo() (string, string, []string, error) {
	return "", "", nil, nil
}
func (f *fakeIpamHandler) GetContainerAddress(string) (string, string, string, error) {
	return "", "", "", nil
}
func (f *fakeIpamHandler) GetInfoByIp(string) (string, string, error) { return "", "", nil }
func (f *fakeIpamHandler) SetForwardInfo(string, int, int, string) error {
	return nil
}
func (f *fakeIpamHandler) GetForwardInfo(string) ([]ipam.ForwardInfo, error) { return nil, nil }
func (f *fakeIpamHandler) GetPoolList() ([]ipam.Pool, error)                 { return nil, nil }
func (f *fakeIpamHandler) GetNetworkInfoById(string) (string, ipam.Allocation, error) {
	return "", ipam.Allocation{}, nil
}
func (f *fakeIpamHandler) GetVethById(string) (string, error) { return "", nil }

type fakePsmHandler struct{}

func (f *fakePsmHandler) StorePod(string, string, string, string, string, string, string, string, string, string, map[string]string, map[string]string) error {
	return nil
}
func (f *fakePsmHandler) StorePodTemplate(string, psm.PodTemplateSpec) error { return nil }
func (f *fakePsmHandler) GetPodTemplate(string) (psm.PodTemplateInfo, error) {
	return psm.PodTemplateInfo{}, nil
}
func (f *fakePsmHandler) GetPodTemplateList() ([]psm.PodTemplateInfo, error) { return nil, nil }
func (f *fakePsmHandler) AddContainerToPodTemplate(string, psm.ContainerTemplateSpec) error {
	return nil
}
func (f *fakePsmHandler) RemovePodTemplate(string) error                   { return nil }
func (f *fakePsmHandler) StoreReplicaSet(string, psm.ReplicaSetSpec) error { return nil }
func (f *fakePsmHandler) GetReplicaSet(string) (psm.ReplicaSetInfo, error) {
	return psm.ReplicaSetInfo{}, nil
}
func (f *fakePsmHandler) GetReplicaSetList() ([]psm.ReplicaSetInfo, error) { return nil, nil }
func (f *fakePsmHandler) IsTemplateReferenced(string) (bool, error)        { return false, nil }
func (f *fakePsmHandler) UpdateReplicaSetReplicas(string, int) error       { return nil }
func (f *fakePsmHandler) RemoveReplicaSet(string) error                    { return nil }
func (f *fakePsmHandler) StoreDeployment(string, psm.DeploymentSpec) error { return nil }
func (f *fakePsmHandler) GetDeployment(string) (psm.DeploymentInfo, error) {
	return psm.DeploymentInfo{}, nil
}
func (f *fakePsmHandler) GetDeploymentList() ([]psm.DeploymentInfo, error) { return nil, nil }
func (f *fakePsmHandler) UpdateDeploymentReplicas(string, int) error       { return nil }
func (f *fakePsmHandler) UpdateDeploymentReplicaSet(string, string) error  { return nil }
func (f *fakePsmHandler) RemoveDeployment(string) error                    { return nil }
func (f *fakePsmHandler) RemovePod(string) error                           { return nil }
func (f *fakePsmHandler) UpdatePod(string, string) error                   { return nil }
func (f *fakePsmHandler) UpdatePodStoppedByUser(string, bool) error        { return nil }
func (f *fakePsmHandler) UpdatePodNamespaces(int, string, string, string, string, string) error {
	return nil
}
func (f *fakePsmHandler) ResetPodNamespaces(string) error        { return nil }
func (f *fakePsmHandler) GetPodList() ([]psm.PodInfo, error)     { return nil, nil }
func (f *fakePsmHandler) GetPodById(string) (psm.PodInfo, error) { return psm.PodInfo{}, nil }
func (f *fakePsmHandler) IsNameAlreadyUsed(string, string) bool  { return false }
func (f *fakePsmHandler) GetPodIdByName(string, string) (string, error) {
	return "", nil
}
func (f *fakePsmHandler) ResolvePodId(string, string) (string, error) { return "", nil }
func (f *fakePsmHandler) IsPodExist(string) bool                      { return false }
func (f *fakePsmHandler) IsPodOwner(string) bool                      { return true }
func (f *fakePsmHandler) GetPodOwnerPid(string) (int, error)          { return 0, nil }

type fakeNetworkService struct{}

func (f *fakeNetworkService) CreateNewNetwork(network.ServiceNewNetworkModel) error { return nil }
func (f *fakeNetworkService) RemoveNetwork(network.ServiceRemoveNetworkModel) error { return nil }
func (f *fakeNetworkService) CreateBridgeInterface(string, string) error            { return nil }
func (f *fakeNetworkService) CreateMasqueradeRule(string, string) error             { return nil }
func (f *fakeNetworkService) InsertInputRule(int, network.InputRuleModel, string) error {
	return nil
}
func (f *fakeNetworkService) CreateForwardingRule(string, network.ServiceNetworkModel) error {
	return nil
}
func (f *fakeNetworkService) CreateRedirectDnsTrafficRule(string, string) error { return nil }
func (f *fakeNetworkService) RemoveRedirectDnsTrafficRule(string, string) error { return nil }
func (f *fakeNetworkService) RemoveForwardingRule(string, network.ServiceNetworkModel) error {
	return nil
}

type fakeFilesystemHandler struct{}

func (f *fakeFilesystemHandler) MkdirAll(string, os.FileMode) error                  { return nil }
func (f *fakeFilesystemHandler) ReadFile(string) ([]byte, error)                     { return nil, nil }
func (f *fakeFilesystemHandler) WriteFile(string, []byte, os.FileMode) error         { return nil }
func (f *fakeFilesystemHandler) Open(string) (*os.File, error)                       { return nil, nil }
func (f *fakeFilesystemHandler) OpenFile(string, int, os.FileMode) (*os.File, error) { return nil, nil }
func (f *fakeFilesystemHandler) Copy(io.Writer, io.Reader) (int64, error)            { return 0, nil }
func (f *fakeFilesystemHandler) Remove(string) error                                 { return nil }
func (f *fakeFilesystemHandler) RemoveAll(string) error                              { return nil }
func (f *fakeFilesystemHandler) Rename(string, string) error                         { return nil }
func (f *fakeFilesystemHandler) IsNotExist(err error) bool                           { return os.IsNotExist(err) }
func (f *fakeFilesystemHandler) Flock(int, int) error                                { return nil }
func (f *fakeFilesystemHandler) Chmod(string, os.FileMode) error                     { return nil }
