package create

import (
	"fmt"
	"time"

	"raind/internal/droplet/hook"
	"raind/internal/droplet/logs"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
)

type Option struct {
	ContainerId   string
	Bundle        string
	ConsoleSocket string
	PidFile       string
	PrintPidFlag  bool
	TtyFlag       bool
}

type Controller struct {
	LoadSpec                          func(containerId string) (spec.Spec, error)
	PrepareBundleConfig               func(containerId string, bundle string) error
	BundlePathForContainer            func(containerId string, bundle string) (string, error)
	RootlessConfigFromSpec            func(containerSpec spec.Spec) (spec.RootlessConfigObject, bool)
	PrepareRootlessShiftedImageLayers func(containerId string, containerSpec spec.Spec, rootlessConfig spec.RootlessConfigObject) (spec.Spec, error)
	RewriteContainerSpecAndHash       func(containerId string, containerSpec spec.Spec) error
	ShouldSkipHostSideSetup           func(containerSpec spec.Spec) bool
	CreateFifo                        func(path string) error
	PrepareRootlessFifo               func(path string, rootlessConfig spec.RootlessConfigObject) error
	PrepareRootlessWritableFilesystem func(containerSpec spec.Spec) error
	CleanupShimFile                   func(containerId string) error
	ExecuteShim                       func(containerId string, containerSpec spec.Spec, fifo string, tty bool, consoleSocket string) (int, error)
	WaitInitPid                       func(containerId string, timeout time.Duration, pollInterval time.Duration) (int, error)
	WrapInitPidWaitError              func(containerId string, err error) error
	WriteContainerPidFile             func(pidFile string, pid int) error
	WriteExternalPidFileMarker        func(containerId string, pidFile string) error
	PrepareCgroup                     func(containerId string, containerSpec spec.Spec, pid int) error
	PrepareNetwork                    func(containerId string, pid int, annotation spec.AnnotationObject) error
	ContainerStatusManager            status.ContainerStatusManager
	ContainerHookController           hook.ContainerHookController
}

func (c *Controller) Create(opt Option) (err error) {
	var (
		containerSpec spec.Spec
		event         = "create"
		stage         string
		pid           int
	)

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
			Spec:        &containerSpec,
			Result:      result,
			Error:       err,
		})
	}()

	stage = "prepare_bundle_config"
	err = c.PrepareBundleConfig(opt.ContainerId, opt.Bundle)
	if err != nil {
		return err
	}

	stage = "load_spec"
	containerSpec, err = c.LoadSpec(opt.ContainerId)
	if err != nil {
		return err
	}
	bundlePath, err := c.BundlePathForContainer(opt.ContainerId, opt.Bundle)
	if err != nil {
		return err
	}
	tty := opt.TtyFlag || containerSpec.Process.Terminal
	if opt.ConsoleSocket != "" && !tty {
		return fmt.Errorf("--console-socket requires --tty or process.terminal=true")
	}

	if rootlessConfig, ok := c.RootlessConfigFromSpec(containerSpec); ok {
		stage = "prepare_rootless_shifted_image_layers"
		containerSpec, err = c.PrepareRootlessShiftedImageLayers(opt.ContainerId, containerSpec, rootlessConfig)
		if err != nil {
			return err
		}

		stage = "rewrite_rootless_spec"
		err = c.RewriteContainerSpecAndHash(opt.ContainerId, containerSpec)
		if err != nil {
			return err
		}
	}
	nestedRootless := c.ShouldSkipHostSideSetup(containerSpec)

	stage = "create_state"
	err = c.ContainerStatusManager.CreateStatusFile(
		opt.ContainerId,
		0,
		status.CREATING,
		containerSpec.Root.Path,
		bundlePath,
		containerSpec.Annotations,
	)
	if err != nil {
		return err
	}

	stage = "hook_create_runtime"
	err = c.ContainerHookController.RunCreateRuntimeHooks(
		opt.ContainerId,
		containerSpec.Hooks.CreateRuntime,
	)
	if err != nil {
		return err
	}

	stage = "create_fifo"
	fifo := utils.FifoPath(opt.ContainerId)
	err = c.CreateFifo(fifo)
	if err != nil {
		return err
	}
	if rootlessConfig, ok := c.RootlessConfigFromSpec(containerSpec); ok {
		stage = "prepare_rootless_fifo"
		err = c.PrepareRootlessFifo(fifo, rootlessConfig)
		if err != nil {
			return err
		}

		stage = "prepare_rootless_writable_filesystem"
		err = c.PrepareRootlessWritableFilesystem(containerSpec)
		if err != nil {
			return err
		}
	}

	var (
		initPid int
		shimPid int
	)
	stage = "cleanup_shim_file"
	err = c.CleanupShimFile(opt.ContainerId)
	if err != nil {
		return err
	}

	stage = "execute_shim"
	pid, err = c.ExecuteShim(opt.ContainerId, containerSpec, fifo, tty, opt.ConsoleSocket)
	if err != nil {
		return err
	}
	shimPid = pid

	stage = "wait_init_pid"
	initPid, err = c.WaitInitPid(opt.ContainerId, 10*time.Second, 20*time.Millisecond)
	if err != nil {
		return c.WrapInitPidWaitError(opt.ContainerId, err)
	}
	pid = initPid
	stage = "write_pid_file"
	err = c.WriteContainerPidFile(opt.PidFile, initPid)
	if err != nil {
		return err
	}
	stage = "write_pid_file_marker"
	err = c.WriteExternalPidFileMarker(opt.ContainerId, opt.PidFile)
	if err != nil {
		return err
	}

	if !nestedRootless {
		stage = "setup_cgroup"
		err = c.PrepareCgroup(opt.ContainerId, containerSpec, initPid)
		if err != nil {
			return c.WrapInitPidWaitError(opt.ContainerId, err)
		}
	}

	if !nestedRootless {
		stage = "setup_network"
		err = c.PrepareNetwork(opt.ContainerId, initPid, containerSpec.Annotations)
		if err != nil {
			return err
		}
	}

	stage = "update_state"
	err = c.ContainerStatusManager.UpdateStatus(
		opt.ContainerId,
		status.CREATED,
		initPid,
		shimPid,
	)
	if err != nil {
		return err
	}
	if len(containerSpec.Hooks.Prestart) > 0 {
		stage = "hook_prestart"
		err = c.ContainerHookController.RunCreateRuntimeHooks(
			opt.ContainerId,
			containerSpec.Hooks.Prestart,
		)
		if err != nil {
			return err
		}
	}

	stage = "hook_create_container"
	err = c.ContainerHookController.RunCreateContainerHooks(
		opt.ContainerId,
		containerSpec.Hooks.CreateContainer,
	)
	if err != nil {
		return err
	}
	return nil
}
