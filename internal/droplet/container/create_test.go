package container

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCreateFifoCreator struct {
	calls []string
	err   error
}

func (f *fakeCreateFifoCreator) createFifo(path string) error {
	f.calls = append(f.calls, path)
	return f.err
}

type fakeCreateProcessExecutor struct {
	initPid int
	shimPid int

	initCalls []createProcessCall
	shimCalls []createProcessCall

	initErr error
	shimErr error
}

type createProcessCall struct {
	containerId   string
	fifo          string
	entrypoint    []string
	tty           bool
	consoleSocket string
}

func (f *fakeCreateProcessExecutor) executeInit(containerId string, containerSpec spec.Spec, fifo string) (int, error) {
	f.initCalls = append(f.initCalls, createProcessCall{
		containerId: containerId,
		fifo:        fifo,
		entrypoint:  append([]string(nil), containerSpec.Process.Args...),
	})
	return f.initPid, f.initErr
}

func (f *fakeCreateProcessExecutor) executeShim(containerId string, containerSpec spec.Spec, fifo string, tty bool, consoleSocket string) (int, error) {
	f.shimCalls = append(f.shimCalls, createProcessCall{
		containerId:   containerId,
		fifo:          fifo,
		entrypoint:    append([]string(nil), containerSpec.Process.Args...),
		tty:           tty,
		consoleSocket: consoleSocket,
	})
	_ = os.MkdirAll(utils.ContainerDir(containerId), 0755)
	_ = os.WriteFile(utils.InitPidFilePath(containerId), []byte("4321\n"), 0644)
	return f.shimPid, f.shimErr
}

type fakeCreateCgroupPreparer struct {
	calls []createPrepareCall
	err   error
}

type createPrepareCall struct {
	containerId string
	pid         int
	spec        spec.Spec
	annotation  spec.AnnotationObject
}

func (f *fakeCreateCgroupPreparer) prepare(containerId string, containerSpec spec.Spec, pid int) error {
	f.calls = append(f.calls, createPrepareCall{
		containerId: containerId,
		pid:         pid,
		spec:        containerSpec,
	})
	return f.err
}

type fakeCreateNetworkPreparer struct {
	calls []createPrepareCall
	err   error
}

func (f *fakeCreateNetworkPreparer) prepare(containerId string, pid int, annotation spec.AnnotationObject) error {
	f.calls = append(f.calls, createPrepareCall{
		containerId: containerId,
		pid:         pid,
		annotation:  annotation,
	})
	return f.err
}

func setupCreateSpecFile(t *testing.T, containerId string, containerSpec spec.Spec) {
	t.Helper()
	require.NoError(t, os.MkdirAll(utils.ContainerDir(containerId), 0755))
	require.NoError(t, utils.WriteJsonToFile(utils.ConfigFilePath(containerId), containerSpec))
}

func minimalCreateSpec() spec.Spec {
	return spec.Spec{
		OciVersion: "1.3.0",
		Root:       spec.RootObject{Path: "/rootfs"},
		Process: spec.ProcessObject{
			Args: []string{"/bin/sh"},
			Env:  []string{"PATH=/bin"},
		},
		Annotations: spec.AnnotationObject{
			Net:   `{"interface":{"name":"","ipv4":{"address":""}}}`,
			Image: `{"rootfsType":"overlay","imageLayer":["/layers/a"],"upperDir":"/upper","workDir":"/work"}`,
		},
		Hooks: spec.HookLifecycleObject{
			CreateRuntime:   []spec.HookObject{{Path: "/bin/create-runtime"}},
			CreateContainer: []spec.HookObject{{Path: "/bin/create-container"}},
		},
		LinuxSpec: spec.LinuxSpecObject{
			Seccomp: &spec.SeccompObject{DefaultAction: "SCMP_ACT_ALLOW"},
		},
	}
}

func TestContainerCreatorCreateRunsLifecyclePipeline(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	containerSpec := minimalCreateSpec()
	setupCreateSpecFile(t, containerId, containerSpec)
	specLoader := &fakeDeleteSpecLoader{spec: containerSpec}
	fifoCreator := &fakeCreateFifoCreator{}
	processExecutor := &fakeCreateProcessExecutor{shimPid: 2222}
	cgroupPreparer := &fakeCreateCgroupPreparer{}
	networkPreparer := &fakeCreateNetworkPreparer{}
	statusManager := &fakeDeleteStatusManager{}
	hookController := &fakeDeleteHookController{}
	creator := &ContainerCreator{
		specLoader:               specLoader,
		fifoCreator:              fifoCreator,
		processExecutor:          processExecutor,
		containerCgroupPreparer:  cgroupPreparer,
		containerNetworkPreparer: networkPreparer,
		containerStatusManager:   statusManager,
		containerHookController:  hookController,
	}

	// == exercise ==
	err := creator.Create(CreateOption{ContainerId: containerId})

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, []string{containerId}, specLoader.calls)
	assert.Equal(t, []deleteCreateStatusCall{{
		containerId: containerId,
		pid:         0,
		status:      status.CREATING,
		rootfs:      "/rootfs",
		bundle:      utils.ContainerDir(containerId),
		annotation:  containerSpec.Annotations,
	}}, statusManager.createdStatuses)
	assert.Equal(t, []deleteHookCall{{containerId: containerId, hooks: containerSpec.Hooks.CreateRuntime}}, hookController.createRuntimeCalls)
	assert.Equal(t, []string{utils.FifoPath(containerId)}, fifoCreator.calls)
	assert.Empty(t, processExecutor.initCalls)
	assert.Equal(t, []createProcessCall{{containerId: containerId, fifo: utils.FifoPath(containerId), entrypoint: []string{"/bin/sh"}, tty: false}}, processExecutor.shimCalls)
	assert.Equal(t, []createPrepareCall{{containerId: containerId, pid: 4321, spec: containerSpec}}, cgroupPreparer.calls)
	assert.Equal(t, []createPrepareCall{{containerId: containerId, pid: 4321, annotation: containerSpec.Annotations}}, networkPreparer.calls)
	assert.Equal(t, []deleteStatusUpdate{{containerId: containerId, status: status.CREATED, pid: 4321, shimPid: 2222}}, statusManager.updates)
	assert.Equal(t, []deleteHookCall{{containerId: containerId, hooks: containerSpec.Hooks.CreateContainer}}, hookController.createContainerCalls)
}

func TestContainerCreatorCreateTTYUsesShimPidAndInitPidFile(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	containerSpec := minimalCreateSpec()
	setupCreateSpecFile(t, containerId, containerSpec)
	processExecutor := &fakeCreateProcessExecutor{shimPid: 2222}
	statusManager := &fakeDeleteStatusManager{}
	creator := &ContainerCreator{
		specLoader:               &fakeDeleteSpecLoader{spec: containerSpec},
		fifoCreator:              &fakeCreateFifoCreator{},
		processExecutor:          processExecutor,
		containerCgroupPreparer:  &fakeCreateCgroupPreparer{},
		containerNetworkPreparer: &fakeCreateNetworkPreparer{},
		containerStatusManager:   statusManager,
		containerHookController:  &fakeDeleteHookController{},
	}

	// == exercise ==
	err := creator.Create(CreateOption{ContainerId: containerId, TtyFlag: true})

	// == assert ==
	require.NoError(t, err)
	assert.Empty(t, processExecutor.initCalls)
	assert.Equal(t, []createProcessCall{{containerId: containerId, fifo: utils.FifoPath(containerId), entrypoint: []string{"/bin/sh"}, tty: true}}, processExecutor.shimCalls)
	assert.Contains(t, statusManager.updates, deleteStatusUpdate{
		containerId: containerId,
		status:      status.CREATED,
		pid:         4321,
		shimPid:     2222,
	})
}

func TestContainerCreatorCreateUsesSpecTerminalForShimTTY(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	containerSpec := minimalCreateSpec()
	containerSpec.Process.Terminal = true
	setupCreateSpecFile(t, containerId, containerSpec)
	processExecutor := &fakeCreateProcessExecutor{shimPid: 2222}
	creator := &ContainerCreator{
		specLoader:               &fakeDeleteSpecLoader{spec: containerSpec},
		fifoCreator:              &fakeCreateFifoCreator{},
		processExecutor:          processExecutor,
		containerCgroupPreparer:  &fakeCreateCgroupPreparer{},
		containerNetworkPreparer: &fakeCreateNetworkPreparer{},
		containerStatusManager:   &fakeDeleteStatusManager{},
		containerHookController:  &fakeDeleteHookController{},
	}

	// == exercise ==
	err := creator.Create(CreateOption{ContainerId: containerId})

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, []createProcessCall{{containerId: containerId, fifo: utils.FifoPath(containerId), entrypoint: []string{"/bin/sh"}, tty: true}}, processExecutor.shimCalls)
}

func TestContainerCreatorCreatePassesConsoleSocketToShim(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	consoleSocket := filepath.Join(t.TempDir(), "console.sock")
	containerSpec := minimalCreateSpec()
	containerSpec.Process.Terminal = true
	setupCreateSpecFile(t, containerId, containerSpec)
	processExecutor := &fakeCreateProcessExecutor{shimPid: 2222}
	creator := &ContainerCreator{
		specLoader:               &fakeDeleteSpecLoader{spec: containerSpec},
		fifoCreator:              &fakeCreateFifoCreator{},
		processExecutor:          processExecutor,
		containerCgroupPreparer:  &fakeCreateCgroupPreparer{},
		containerNetworkPreparer: &fakeCreateNetworkPreparer{},
		containerStatusManager:   &fakeDeleteStatusManager{},
		containerHookController:  &fakeDeleteHookController{},
	}

	// == exercise ==
	err := creator.Create(CreateOption{ContainerId: containerId, ConsoleSocket: consoleSocket})

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, []createProcessCall{{
		containerId:   containerId,
		fifo:          utils.FifoPath(containerId),
		entrypoint:    []string{"/bin/sh"},
		tty:           true,
		consoleSocket: consoleSocket,
	}}, processExecutor.shimCalls)
}

func TestContainerCreatorCreateRejectsConsoleSocketWithoutTTY(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	containerSpec := minimalCreateSpec()
	setupCreateSpecFile(t, containerId, containerSpec)
	processExecutor := &fakeCreateProcessExecutor{shimPid: 2222}
	creator := &ContainerCreator{
		specLoader:               &fakeDeleteSpecLoader{spec: containerSpec},
		fifoCreator:              &fakeCreateFifoCreator{},
		processExecutor:          processExecutor,
		containerCgroupPreparer:  &fakeCreateCgroupPreparer{},
		containerNetworkPreparer: &fakeCreateNetworkPreparer{},
		containerStatusManager:   &fakeDeleteStatusManager{},
		containerHookController:  &fakeDeleteHookController{},
	}

	// == exercise ==
	err := creator.Create(CreateOption{ContainerId: containerId, ConsoleSocket: filepath.Join(t.TempDir(), "console.sock")})

	// == assert ==
	require.Error(t, err)
	assert.Contains(t, err.Error(), "console-socket requires")
	assert.Empty(t, processExecutor.shimCalls)
}

func TestContainerCreatorCreateWithBundleStoresBundleAndWritesPidFile(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	bundle := t.TempDir()
	pidFile := filepath.Join(t.TempDir(), "container.pid")
	containerSpec := minimalCreateSpec()
	containerSpec.Root.Path = "rootfs"
	require.NoError(t, os.MkdirAll(filepath.Join(bundle, "rootfs"), 0755))
	require.NoError(t, utils.WriteJsonToFile(filepath.Join(bundle, "config.json"), containerSpec))
	resolvedSpec := containerSpec
	resolvedSpec.Root.Path = filepath.Join(bundle, "rootfs")
	processExecutor := &fakeCreateProcessExecutor{shimPid: 2222}
	statusManager := &fakeDeleteStatusManager{}
	creator := &ContainerCreator{
		specLoader:               &fakeDeleteSpecLoader{spec: resolvedSpec},
		fifoCreator:              &fakeCreateFifoCreator{},
		processExecutor:          processExecutor,
		containerCgroupPreparer:  &fakeCreateCgroupPreparer{},
		containerNetworkPreparer: &fakeCreateNetworkPreparer{},
		containerStatusManager:   statusManager,
		containerHookController:  &fakeDeleteHookController{},
	}

	// == exercise ==
	err := creator.Create(CreateOption{ContainerId: containerId, Bundle: bundle, PidFile: pidFile})

	// == assert ==
	require.NoError(t, err)
	require.Len(t, statusManager.createdStatuses, 1)
	assert.Equal(t, bundle, statusManager.createdStatuses[0].bundle)
	data, err := os.ReadFile(pidFile)
	require.NoError(t, err)
	assert.Equal(t, "4321", string(data))
	marker, err := os.ReadFile(utils.ExternalPidFileMarkerPath(containerId))
	require.NoError(t, err)
	assert.Equal(t, pidFile, string(marker))
}

func TestContainerCreatorSpecSecureLoadWritesHashAndReturnsSpec(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	containerSpec := minimalCreateSpec()
	setupCreateSpecFile(t, containerId, containerSpec)
	creator := &ContainerCreator{
		specLoader: &fakeDeleteSpecLoader{spec: containerSpec},
	}

	// == exercise ==
	got, err := creator.specSecureLoad(containerId)

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, containerSpec, got)
	var hash spec.SpecHash
	require.NoError(t, utils.ReadJsonFile(utils.ConfigFileHashPath(containerId), &hash))
	assert.NotEmpty(t, hash.Sha256)
}

func TestContainerCreatorSpecSecureLoadDetectsConfigChangeDuringLoad(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	containerSpec := minimalCreateSpec()
	setupCreateSpecFile(t, containerId, containerSpec)
	specLoader := &fakeDeleteSpecLoader{spec: containerSpec}
	specLoader.onLoad = func(containerId string) {
		require.NoError(t, os.WriteFile(utils.ConfigFilePath(containerId), []byte(`{"changed":true}`), 0644))
	}
	creator := &ContainerCreator{specLoader: specLoader}

	// == exercise ==
	_, err := creator.specSecureLoad(containerId)

	// == assert ==
	require.Error(t, err)
	assert.Contains(t, err.Error(), "hash validation failed")
}

func TestContainerCreatorCleanupShimFileRemovesSocketAndPidFile(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	require.NoError(t, os.MkdirAll(utils.ContainerDir(containerId), 0755))
	require.NoError(t, os.WriteFile(utils.SockPath(containerId), []byte("sock"), 0644))
	require.NoError(t, os.WriteFile(utils.InitPidFilePath(containerId), []byte("1234\n"), 0644))
	creator := &ContainerCreator{}

	// == exercise ==
	err := creator.cleanupShimFile(containerId)

	// == assert ==
	require.NoError(t, err)
	assert.NoFileExists(t, utils.SockPath(containerId))
	assert.NoFileExists(t, utils.InitPidFilePath(containerId))
}

func TestContainerCreatorWaitInitPidReturnsDelayedPidFile(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	require.NoError(t, os.MkdirAll(utils.ContainerDir(containerId), 0755))
	creator := &ContainerCreator{}
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = os.WriteFile(utils.InitPidFilePath(containerId), []byte("4242\n"), 0644)
	}()

	// == exercise ==
	pid, err := creator.waitInitPid(containerId, time.Second, time.Millisecond)

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, 4242, pid)
}

func TestContainerCreatorWaitInitPidTimesOut(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	require.NoError(t, os.MkdirAll(utils.ContainerDir(containerId), 0755))
	creator := &ContainerCreator{}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	// == exercise ==
	pid, err := creator.waitInitPidContext(ctx, containerId, time.Millisecond)

	// == assert ==
	require.Error(t, err)
	assert.Equal(t, -1, pid)
	assert.Contains(t, err.Error(), "wait init pid timeout")
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()

	fn()
	require.NoError(t, w.Close())
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	return buf.String()
}

func TestContainerCreatorCreateStopsWhenCgroupPrepareFails(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	containerSpec := minimalCreateSpec()
	setupCreateSpecFile(t, containerId, containerSpec)
	cgroupPreparer := &fakeCreateCgroupPreparer{err: errors.New("cgroup failed")}
	statusManager := &fakeDeleteStatusManager{}
	hookController := &fakeDeleteHookController{}
	creator := &ContainerCreator{
		specLoader:               &fakeDeleteSpecLoader{spec: containerSpec},
		fifoCreator:              &fakeCreateFifoCreator{},
		processExecutor:          &fakeCreateProcessExecutor{initPid: 1234},
		containerCgroupPreparer:  cgroupPreparer,
		containerNetworkPreparer: &fakeCreateNetworkPreparer{},
		containerStatusManager:   statusManager,
		containerHookController:  hookController,
	}

	// == exercise ==
	err := creator.Create(CreateOption{ContainerId: containerId})

	// == assert ==
	require.Error(t, err)
	assert.Equal(t, "cgroup failed", err.Error())
	assert.Empty(t, statusManager.updates)
	assert.Empty(t, hookController.createContainerCalls)
}
