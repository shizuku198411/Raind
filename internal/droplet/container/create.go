package container

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	createop "raind/internal/droplet/container/create"
	"raind/internal/droplet/hook"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
)

// NewContainerCreator constructs a ContainerCreator with the default
// implementations of its dependencies (SpecLoader, FifoCreator,
// ProcessExecutor, network/cgroup preparers, status manager, hook controller).
// This acts as the main entry point for the container creation workflow.
func NewContainerCreator() *ContainerCreator {
	return &ContainerCreator{
		specLoader:               newFileSpecLoader(),
		fifoCreator:              newContainerFifoHandler(),
		processExecutor:          newContainerInitExecutor(),
		containerNetworkPreparer: newContainerNetworkController(),
		containerCgroupPreparer:  newContainerCgroupController(),
		containerStatusManager:   status.NewStatusHandler(),
		containerHookController:  hook.NewHookController(),
	}
}

// newContainerInitExecutor constructs a containerInitExecutor with the default
// command factory. It is used as the default implementation for spawning
// the container init process workflow.
func newContainerInitExecutor() *containerInitExecutor {
	return &containerInitExecutor{
		commandFactory: &utils.ExecCommandFactory{},
		syscallHandler: utils.NewSyscallHandler(),
	}
}

// ContainerCreator orchestrates the container creation flow for a single
// container instance.
//
// The flow currently consists of:
//
//  1. Loading the OCI spec (config.json)
//  2. Creating the initial state.json (status=creating, pid=0)
//  3. Running createRuntime hooks
//  4. Creating the FIFO used for init synchronization
//  5. Launching the init process via the init subcommand
//  6. Configuring cgroups for the init process
//  7. Configuring network for the init process
//  8. Updating state.json (status=created, pid=init pid)
//  9. Running createContainer hooks
//
// Each step is delegated to an interface to allow testing and substitution.
type ContainerCreator struct {
	specLoader               specLoader
	fifoCreator              fifoCreator
	processExecutor          processExecutor
	containerNetworkPreparer containerNetworkPreparer
	containerCgroupPreparer  containerCgroupPreparer
	containerStatusManager   status.ContainerStatusManager
	containerHookController  hook.ContainerHookController
}

// Create executes the container creation pipeline for the given container ID.
//
// It coordinates the high-level workflow by:
//   - Loading the spec
//   - Initializing container state
//   - Running lifecycle hooks
//   - Spawning the init process
//   - Applying cgroup and network configuration
//   - Updating final status
//
// This method performs no low-level work itself and relies entirely on
// its collaborators. If any step fails, the error is returned immediately.
func (c *ContainerCreator) Create(opt CreateOption) (err error) {
	controller := createop.Controller{
		LoadSpec:               c.specSecureLoad,
		PrepareBundleConfig:    prepareBundleConfig,
		BundlePathForContainer: bundlePathForContainer,
		RootlessConfigFromSpec: rootlessConfigFromSpec,
		PrepareRootlessShiftedImageLayers: func(containerId string, containerSpec spec.Spec, rootlessConfig spec.RootlessConfigObject) (spec.Spec, error) {
			return prepareRootlessShiftedImageLayers(containerId, containerSpec, rootlessConfig)
		},
		RewriteContainerSpecAndHash:       rewriteContainerSpecAndHash,
		ShouldSkipHostSideSetup:           shouldSkipHostSideSetupForNestedRootless,
		CreateFifo:                        c.fifoCreator.createFifo,
		PrepareRootlessFifo:               prepareRootlessFifo,
		PrepareRootlessWritableFilesystem: prepareRootlessWritableFilesystem,
		CleanupShimFile:                   c.cleanupShimFile,
		ExecuteShim:                       c.processExecutor.executeShim,
		WaitInitPid:                       c.waitInitPid,
		WrapInitPidWaitError:              c.wrapInitPidWaitError,
		WriteContainerPidFile:             writeContainerPidFile,
		WriteExternalPidFileMarker:        writeExternalPidFileMarker,
		PrepareCgroup:                     c.containerCgroupPreparer.prepare,
		PrepareNetwork:                    c.containerNetworkPreparer.prepare,
		ContainerStatusManager:            c.containerStatusManager,
		ContainerHookController:           c.containerHookController,
	}
	return controller.Create(createop.Option{
		ContainerId:   opt.ContainerId,
		Bundle:        opt.Bundle,
		ConsoleSocket: opt.ConsoleSocket,
		PidFile:       opt.PidFile,
		PrintPidFlag:  opt.PrintPidFlag,
		TtyFlag:       opt.TtyFlag,
	})
}

func shouldSkipHostSideSetupForNestedRootless(containerSpec spec.Spec) bool {
	return isRootlessSpec(containerSpec) && currentUserNamespaceDiffersFromInit()
}

func (c *ContainerCreator) specSecureLoad(containerId string) (spec.Spec, error) {
	return sealAndLoadSpec(containerId, c.specLoader)
}

// processExecutor defines the behavior for spawning the container init process.
//
// It is an interface so that the behavior can be mocked in tests and
// replaced by alternative implementations if needed.
type processExecutor interface {
	executeShim(containerId string, spec spec.Spec, fifo string, tty bool, consoleSocket string) (int, error)
}

// containerInitExecutor is the default implementation of processExecutor.
//
// It invokes this binary with the `init` subcommand and the FIFO path,
// passing the spec's process args as the container entrypoint.
type containerInitExecutor struct {
	commandFactory utils.CommandFactory
	syscallHandler utils.KernelSyscallHandler
}

// executeInit starts the init process and returns its PID.
//
// The init process is started as a child of the current runtime binary
// with the appropriate namespace and process attributes applied. The FIFO
// path is passed as an argument so that the init process can synchronize
// with the runtime before proceeding.
func (c *containerInitExecutor) executeInit(containerId string, spec spec.Spec, fifo string) (int, error) {
	// retrieve entrypoint from spec
	entrypoint := spec.Process.Args

	// prepare init subcommand
	initArgs := append([]string{"init", containerId, fifo}, entrypoint...)
	cmd := c.commandFactory.Command(utils.SelfBinPath(), initArgs...)
	// set stdout/stderr to log files
	logPath := utils.ContainerLogPath(containerId)
	f, err := c.syscallHandler.OpenFile(logPath, os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return -1, err
	}
	if rootlessConfig, ok := rootlessConfigFromSpec(spec); ok {
		if err := prepareRootlessInitLog(logPath, rootlessConfig); err != nil {
			_ = f.Close()
			return -1, err
		}
	}
	cmd.SetStdout(f)
	cmd.SetStderr(f)

	// apply SysProcAttr
	procAttr := buildProcAttrForContainer(spec)
	sysProcAttr := buildSysProcAttr(procAttr)
	sysProcAttr.Setsid = true
	cmd.SetSysProcAttr(sysProcAttr)

	// execute init subcommand
	if err := cmd.Start(); err != nil {
		return -1, err
	}

	return cmd.Pid(), nil
}

func prepareRootlessInitLog(path string, rootlessConfig spec.RootlessConfigObject) error {
	if currentUserNamespaceDiffersFromInit() {
		return nil
	}
	uid, gid := rootlessHostRootID(rootlessConfig)
	return os.Chown(path, uid, gid)
}

func (c *containerInitExecutor) executeShim(containerId string, spec spec.Spec, fifo string, tty bool, consoleSocket string) (int, error) {
	// retrieve entrypoint from spec
	entrypoint := spec.Process.Args

	// prepare shim subcommand
	shimArgs := []string{"shim"}
	if tty {
		shimArgs = append(shimArgs, "--tty")
	}
	if consoleSocket != "" {
		shimArgs = append(shimArgs, "--console-socket", consoleSocket)
	}
	shimArgs = append(shimArgs, containerId, fifo)
	shimArgs = append(shimArgs, entrypoint...)
	cmd := c.commandFactory.Command(utils.SelfBinPath(), shimArgs...)
	if isOCIBundleMode(containerId) {
		cmd.SetStdout(os.Stdout)
		cmd.SetStderr(os.Stderr)
	}

	// execute shim subcommand
	if err := cmd.Start(); err != nil {
		return -1, err
	}

	return cmd.Pid(), nil
}

func (c *ContainerCreator) waitInitPid(containerId string, timeout time.Duration, pollInterval time.Duration) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return c.waitInitPidContext(ctx, containerId, pollInterval)
}

func (c *ContainerCreator) wrapInitPidWaitError(containerId string, err error) error {
	details := []string{}
	if tail := tailFileForError(utils.ShimLogPath(containerId), 12); tail != "" {
		details = append(details, "shim log:\n"+tail)
	}
	if tail := tailFileForError(utils.ContainerLogPath(containerId), 12); tail != "" {
		details = append(details, "container log:\n"+tail)
	}
	if len(details) == 0 {
		return err
	}
	return fmt.Errorf("%w\n%s", err, strings.Join(details, "\n"))
}

func tailFileForError(path string, maxLines int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return ""
	}
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n")
}

// WaitInitPidContext is the context-aware variant.
func (c *ContainerCreator) waitInitPidContext(ctx context.Context, containerId string, pollInterval time.Duration) (int, error) {
	if pollInterval <= 0 {
		pollInterval = 20 * time.Millisecond
	}

	pidPath := utils.InitPidFilePath(containerId)

	// Use ticker for polling.
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Try immediately once (fast path).
	if pid, ok := c.tryReadPidFile(pidPath); ok {
		return pid, nil
	}

	for {
		select {
		case <-ctx.Done():
			// include last error context if desired; minimal version keeps it simple
			return -1, fmt.Errorf("wait init pid timeout: %w", ctx.Err())
		case <-ticker.C:
			if pid, ok := c.tryReadPidFile(pidPath); ok {
				return pid, nil
			}
		}
	}
}

// tryReadPidFile reads pidfile and parses an int PID.
// Returns (pid, true) only when fully valid.
// Any transient failure returns (_, false).
func (c *ContainerCreator) tryReadPidFile(path string) (int, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return -1, false
	}

	s := strings.TrimSpace(string(b))
	if s == "" {
		return -1, false
	}

	pid64, err := strconv.ParseInt(s, 10, 0)
	if err != nil {
		return -1, false
	}
	pid := int(pid64)
	if pid <= 0 {
		return -1, false
	}

	return pid, true
}

func (c *ContainerCreator) cleanupShimFile(containerId string) error {
	// remove sockefile
	_ = os.Remove(utils.SockPath(containerId))
	// remove pid file
	_ = os.Remove(utils.InitPidFilePath(containerId))
	return nil
}
