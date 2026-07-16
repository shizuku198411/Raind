package create

import (
	"fmt"
	"time"

	"raind/internal/droplet/container/audit"
	"raind/internal/droplet/container/rootless"
	"raind/internal/droplet/container/statusflow"
	"raind/internal/droplet/hook"
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
	LoadSpec                func(containerId string) (spec.Spec, error)
	PrepareBundleConfig     func(containerId string, bundle string) error
	BundlePathForContainer  func(containerId string, bundle string) (string, error)
	CreateFifo              func(path string) error
	RootlessPreparer        RootlessPreparer
	InitSupervisor          InitSupervisor
	HostResourcePreparer    HostResourcePreparer
	ContainerStatusManager  status.ContainerStatusManager
	ContainerHookController hook.ContainerHookController
}

type RootlessPreparer struct {
	BuildPlan                 func(containerSpec spec.Spec) rootless.Plan
	PrepareShiftedImageLayers func(containerId string, containerSpec spec.Spec, rootlessConfig spec.RootlessConfigObject) (spec.Spec, error)
	RewriteSpecAndHash        func(containerId string, containerSpec spec.Spec) error
	PrepareFifo               func(path string, rootlessConfig spec.RootlessConfigObject) error
	PrepareWritableFilesystem func(containerSpec spec.Spec) error
}

type RootlessPlan = rootless.Plan

type InitSupervisor struct {
	CleanupShimFile            func(containerId string) error
	ExecuteShim                func(containerId string, containerSpec spec.Spec, fifo string, tty bool, consoleSocket string) (int, error)
	WaitInitPid                func(containerId string, timeout time.Duration, pollInterval time.Duration) (int, error)
	WrapInitPidWaitError       func(containerId string, err error) error
	WriteContainerPidFile      func(pidFile string, pid int) error
	WriteExternalPidFileMarker func(containerId string, pidFile string) error
}

type InitProcess struct {
	InitPid int
	ShimPid int
}

type HostResourcePreparer struct {
	PrepareCgroup        func(containerId string, containerSpec spec.Spec, pid int) error
	PrepareNetwork       func(containerId string, pid int, annotation spec.AnnotationObject) error
	WrapInitPidWaitError func(containerId string, err error) error
}

func (c *Controller) Create(opt Option) (err error) {
	var containerSpec spec.Spec

	auditLog := audit.New(opt.ContainerId, "create")
	auditLog.SetSpec(&containerSpec)
	defer auditLog.Record(&err)

	auditLog.Stage("prepare_bundle_config")
	err = c.PrepareBundleConfig(opt.ContainerId, opt.Bundle)
	if err != nil {
		return err
	}

	auditLog.Stage("load_spec")
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

	var rootlessPlan RootlessPlan
	containerSpec, rootlessPlan, err = c.RootlessPreparer.PrepareSpec(opt.ContainerId, containerSpec, auditLog.Stage)
	if err != nil {
		return err
	}

	auditLog.Stage("create_state")
	err = statusflow.Create(
		c.ContainerStatusManager,
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

	auditLog.Stage("hook_create_runtime")
	err = c.ContainerHookController.RunCreateRuntimeHooks(
		opt.ContainerId,
		containerSpec.Hooks.CreateRuntime,
	)
	if err != nil {
		return err
	}

	auditLog.Stage("create_fifo")
	fifo := utils.FifoPath(opt.ContainerId)
	err = c.CreateFifo(fifo)
	if err != nil {
		return err
	}
	err = c.RootlessPreparer.PrepareRuntime(fifo, containerSpec, rootlessPlan, auditLog.Stage)
	if err != nil {
		return err
	}

	initProcess, err := c.InitSupervisor.StartAndWait(opt.ContainerId, containerSpec, fifo, tty, opt.ConsoleSocket, opt.PidFile, auditLog.Stage)
	if err != nil {
		return err
	}
	auditLog.SetPid(initProcess.InitPid)

	err = c.HostResourcePreparer.Prepare(opt.ContainerId, containerSpec, initProcess.InitPid, rootlessPlan, auditLog.Stage)
	if err != nil {
		return err
	}

	auditLog.Stage("update_state")
	err = statusflow.Transition(
		c.ContainerStatusManager,
		opt.ContainerId,
		status.CREATED,
		initProcess.InitPid,
		initProcess.ShimPid,
	)
	if err != nil {
		return err
	}
	if len(containerSpec.Hooks.Prestart) > 0 {
		auditLog.Stage("hook_prestart")
		err = c.ContainerHookController.RunCreateRuntimeHooks(
			opt.ContainerId,
			containerSpec.Hooks.Prestart,
		)
		if err != nil {
			return err
		}
	}

	auditLog.Stage("hook_create_container")
	err = c.ContainerHookController.RunCreateContainerHooks(
		opt.ContainerId,
		containerSpec.Hooks.CreateContainer,
	)
	if err != nil {
		return err
	}
	return nil
}

func (p RootlessPreparer) PrepareSpec(containerId string, containerSpec spec.Spec, setStage func(string)) (spec.Spec, RootlessPlan, error) {
	plan := p.BuildPlan(containerSpec)
	if !plan.Enabled {
		return containerSpec, plan, nil
	}

	setStage("prepare_rootless_shifted_image_layers")
	updatedSpec, err := p.PrepareShiftedImageLayers(containerId, containerSpec, plan.Config)
	if err != nil {
		return spec.Spec{}, RootlessPlan{}, err
	}

	setStage("rewrite_rootless_spec")
	if err := p.RewriteSpecAndHash(containerId, updatedSpec); err != nil {
		return spec.Spec{}, RootlessPlan{}, err
	}

	return updatedSpec, plan, nil
}

func (p RootlessPreparer) PrepareRuntime(fifo string, containerSpec spec.Spec, plan RootlessPlan, setStage func(string)) error {
	if !plan.Enabled {
		return nil
	}

	setStage("prepare_rootless_fifo")
	if err := p.PrepareFifo(fifo, plan.Config); err != nil {
		return err
	}

	setStage("prepare_rootless_writable_filesystem")
	return p.PrepareWritableFilesystem(containerSpec)
}

func (s InitSupervisor) StartAndWait(containerId string, containerSpec spec.Spec, fifo string, tty bool, consoleSocket string, pidFile string, setStage func(string)) (InitProcess, error) {
	setStage("cleanup_shim_file")
	if err := s.CleanupShimFile(containerId); err != nil {
		return InitProcess{}, err
	}

	setStage("execute_shim")
	shimPid, err := s.ExecuteShim(containerId, containerSpec, fifo, tty, consoleSocket)
	if err != nil {
		return InitProcess{}, err
	}

	setStage("wait_init_pid")
	initPid, err := s.WaitInitPid(containerId, 10*time.Second, 20*time.Millisecond)
	if err != nil {
		return InitProcess{}, s.WrapInitPidWaitError(containerId, err)
	}

	setStage("write_pid_file")
	if err := s.WriteContainerPidFile(pidFile, initPid); err != nil {
		return InitProcess{}, err
	}

	setStage("write_pid_file_marker")
	if err := s.WriteExternalPidFileMarker(containerId, pidFile); err != nil {
		return InitProcess{}, err
	}

	return InitProcess{InitPid: initPid, ShimPid: shimPid}, nil
}

func (p HostResourcePreparer) Prepare(containerId string, containerSpec spec.Spec, initPid int, rootlessPlan RootlessPlan, setStage func(string)) error {
	if !rootlessPlan.ShouldPrepareHostResources() {
		return nil
	}

	setStage("setup_cgroup")
	if err := p.PrepareCgroup(containerId, containerSpec, initPid); err != nil {
		return p.WrapInitPidWaitError(containerId, err)
	}

	setStage("setup_network")
	return p.PrepareNetwork(containerId, initPid, containerSpec.Annotations)
}
