package container

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"droplet/internal/hook"
	"droplet/internal/logs"
	"droplet/internal/spec"
	"droplet/internal/status"
	"droplet/internal/utils"
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
	var (
		spec  spec.Spec
		event = "create"
		stage string
		pid   int
	)

	// audit log
	defer func() {
		result := "success"
		if err != nil {
			result = "fail"
		}
		_ = logs.RecordAuditLog(logs.AuditRecord{
			ContainerId: opt.ContainerId,
			Event:       event,
			Stage:       stage,
			Pid:         pid,
			Spec:        &spec,
			Result:      result,
			Error:       err,
		})
	}()

	// 1. load config.json
	stage = "load_spec"
	spec, err = c.specSecureLoad(opt.ContainerId)
	if err != nil {
		return err
	}

	// 2. create state.json
	//      status = creating
	//      pid = 0
	stage = "create_state"
	err = c.containerStatusManager.CreateStatusFile(
		opt.ContainerId,
		0,
		status.CREATING,
		spec.Root.Path,
		utils.ContainerDir(opt.ContainerId),
		spec.Annotations,
	)
	if err != nil {
		return err
	}

	// 3. HOOK: createRuntime
	stage = "hook_create_runtime"
	err = c.containerHookController.RunCreateRuntimeHooks(
		opt.ContainerId,
		spec.Hooks.CreateRuntime,
	)
	if err != nil {
		return err
	}

	// 4. create fifo
	stage = "create_fifo"
	fifo := utils.FifoPath(opt.ContainerId)
	err = c.fifoCreator.createFifo(fifo)
	if err != nil {
		return err
	}

	// 5. execute init subcommand
	var (
		initPid int
		shimPid int
	)
	if opt.TtyFlag {
		// cleanup old files before execute shim
		stage = "cleanup_shim_file"
		err = c.cleanupShimFile(opt.ContainerId)
		if err != nil {
			return err
		}

		stage = "execute_shim"
		pid, err = c.processExecutor.executeShim(opt.ContainerId, spec, fifo)
		if err != nil {
			return err
		}
		shimPid = pid

		// wait for pidfile from shim
		stage = "wait_init_pid"
		initPid, err = c.waitInitPid(opt.ContainerId, 3*time.Second, 20*time.Millisecond)
		if err != nil {
			return err
		}
		pid = initPid
	} else {
		stage = "execute_init"
		pid, err = c.processExecutor.executeInit(opt.ContainerId, spec, fifo)
		if err != nil {
			return err
		}
		initPid = pid
	}

	// 6. cgroup setup
	stage = "setup_cgroup"
	err = c.containerCgroupPreparer.prepare(opt.ContainerId, spec, initPid)
	if err != nil {
		return err
	}

	// 7. network setup
	stage = "setup_network"
	err = c.containerNetworkPreparer.prepare(opt.ContainerId, initPid, spec.Annotations)
	if err != nil {
		return err
	}

	// 8. update state.json
	//      status = created
	//      pid    = init pid
	stage = "update_state"
	err = c.containerStatusManager.UpdateStatus(
		opt.ContainerId,
		status.CREATED,
		initPid,
		shimPid,
	)
	if err != nil {
		return err
	}

	// 9. HOOK: createContainer
	stage = "hook_create_container"
	err = c.containerHookController.RunCreateContainerHooks(
		opt.ContainerId,
		spec.Hooks.CreateContainer,
	)
	if err != nil {
		return err
	}
	return nil
}

func (c *ContainerCreator) specSecureLoad(containerId string) (spec.Spec, error) {
	fileHashPath := utils.ConfigFileHashPath(containerId)

	// 1. calculate current config.json file hash
	beforeLoadedHash, err := utils.Sha256File(utils.ConfigFilePath(containerId))
	if err != nil {
		return spec.Spec{}, err
	}

	// 2. write to file
	if err := utils.WriteJsonToFile(
		fileHashPath,
		spec.SpecHash{
			Sha256: beforeLoadedHash,
		},
	); err != nil {
		return spec.Spec{}, err
	}

	// 3. load config.json
	specFile, err := c.specLoader.loadFile(containerId)
	if err != nil {
		return spec.Spec{}, err
	}

	// 4. re-calculate config.json file hash
	afterLoadedHash, err := utils.Sha256File(utils.ConfigFilePath(containerId))
	if err != nil {
		return spec.Spec{}, err
	}

	// 5. load file sha256 from file
	var specFileHash spec.SpecHash
	if err := utils.ReadJsonFile(
		fileHashPath,
		&specFileHash,
	); err != nil {
		return spec.Spec{}, err
	}

	// 6. assert
	// protect hash value tampering
	if beforeLoadedHash != specFileHash.Sha256 {
		return spec.Spec{}, fmt.Errorf("config.json hash validation failed: expect=%s, got=%s", beforeLoadedHash, specFileHash.Sha256)
	}
	// protect config.json tampering
	if specFileHash.Sha256 != afterLoadedHash {
		return spec.Spec{}, fmt.Errorf("config.json hash validation failed: expect=%s, got=%s", specFileHash.Sha256, afterLoadedHash)
	}

	return specFile, nil
}

// processExecutor defines the behavior for spawning the container init process.
//
// It is an interface so that the behavior can be mocked in tests and
// replaced by alternative implementations if needed.
type processExecutor interface {
	executeInit(containerId string, spec spec.Spec, fifo string) (int, error)
	executeShim(containerId string, spec spec.Spec, fifo string) (int, error)
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
	cmd := c.commandFactory.Command(os.Args[0], initArgs...)
	// set stdout/stderr to log files
	logPath := utils.InitLogPath(containerId)
	f, err := c.syscallHandler.OpenFile(logPath, os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return -1, err
	}
	cmd.SetStdout(f)
	cmd.SetStderr(f)

	// apply SysProcAttr
	nsConfig := buildNamespaceConfig(spec)
	procAttr := buildProcAttrForRootContainer(nsConfig)
	sysProcAttr := buildSysProcAttr(procAttr)
	sysProcAttr.Setsid = true
	cmd.SetSysProcAttr(sysProcAttr)

	// execute init subcommand
	if err := cmd.Start(); err != nil {
		return -1, err
	}

	return cmd.Pid(), nil
}

func (c *containerInitExecutor) executeShim(containerId string, spec spec.Spec, fifo string) (int, error) {
	// retrieve entrypoint from spec
	entrypoint := spec.Process.Args

	// prepare shim subcommand
	shimArgs := append([]string{"shim", containerId, fifo}, entrypoint...)
	cmd := c.commandFactory.Command(os.Args[0], shimArgs...)

	// execute init subcommand
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
