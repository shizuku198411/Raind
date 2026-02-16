package container

import (
	"droplet/internal/hook"
	"droplet/internal/status"
	"droplet/internal/utils"
	"fmt"
	"os"
)

// NewContainerRun constructs a ContainerRun using the default
// implementations of its dependencies.
//
// This is the main entry point for running a container in a
// foreground/attached mode, similar to `runc run`.
// Unlike `create` + `start`, this function starts the init process,
// sends the FIFO start signal, and then waits for the container
// process to exit.
func NewContainerRun() *ContainerRun {
	return &ContainerRun{
		specLoader:               newFileSpecLoader(),
		fifoCreator:              newContainerFifoHandler(),
		commandFactory:           utils.NewCommandFactory(),
		containerStart:           NewContainerStart(),
		containerCgroupPreparer:  newContainerCgroupController(),
		containerNetworkPreparer: newContainerNetworkController(),
		containerStatusManager:   status.NewStatusHandler(),
		containerHookController:  hook.NewHookController(),
	}
}

// ContainerRun orchestrates the "run" lifecycle of a container.
//
// The run flow performs the following steps:
//
//  1. Load the OCI spec (config.json)
//  2. Create the FIFO used for init synchronization
//  3. Spawn the init subprocess of this runtime (via the `init` subcommand)
//  4. Signal the init process to start by writing to the FIFO
//  5. Attach to and wait for the container process to exit
//
// This differs from the `create` + `start` workflow in that the caller
// remains attached to the container process and blocks until it terminates.
type ContainerRun struct {
	specLoader               specLoader
	fifoCreator              fifoCreator
	commandFactory           utils.CommandFactory
	containerStart           *ContainerStart
	containerCgroupPreparer  containerCgroupPreparer
	containerNetworkPreparer containerNetworkPreparer
	containerStatusManager   status.ContainerStatusManager
	containerHookController  hook.ContainerHookController
}

// Run executes the container run pipeline for the provided container ID.
//
// The entrypoint specified in the OCI spec's process section is executed
// inside the init process after synchronization via FIFO.
//
// On success, this method blocks until the container process exits and
// returns the exit status of the process. Any failure during startup or
// synchronization results in an error being returned.
func (c *ContainerRun) Run(opt RunOption) error {
	// 1. load config.json
	spec, err := c.specLoader.loadFile(opt.ContainerId)
	if err != nil {
		return err
	}

	// 2. create state.json
	//      status = creating
	//      pid = 0
	if err := c.containerStatusManager.CreateStatusFile(
		opt.ContainerId,
		0,
		status.CREATING,
		spec.Root.Path,
		utils.ContainerDir(opt.ContainerId),
		spec.Annotations,
	); err != nil {
		return err
	}

	// 3. HOOK: createRuntime
	if err := c.containerHookController.RunCreateRuntimeHooks(
		opt.ContainerId,
		spec.Hooks.CreateRuntime,
	); err != nil {
		return err
	}

	// 4. create fifo
	fifo := utils.FifoPath(opt.ContainerId)
	if err := c.fifoCreator.createFifo(fifo); err != nil {
		return err
	}

	// 5. prepare init subcommand
	entrypoint := spec.Process.Args
	initArgs := append([]string{"init", opt.ContainerId, fifo}, entrypoint...)
	cmd := c.commandFactory.Command(os.Args[0], initArgs...)
	// set stdout/stderr/stdin
	if opt.Tty {
		cmd.SetStdout(os.Stdout)
		cmd.SetStderr(os.Stderr)
		cmd.SetStdin(os.Stdin)
	}
	// TODO: non-interactive mode
	// when started in non-interactive mode, set stdout/stderr to log files

	// apply SysProcAttr
	nsConfig := buildNamespaceConfig(spec)
	procAttr := buildProcAttrForRootContainer(nsConfig)
	sysProcAttr := buildSysProcAttr(procAttr)
	cmd.SetSysProcAttr(sysProcAttr)

	// 6. start init process
	if err := cmd.Start(); err != nil {
		return err
	}
	initPid := cmd.Pid()

	// output when init process has been created
	// if --print-pid is setted, print message with pid
	// otherwise print message with Container ID
	if opt.PrintPidFlag {
		fmt.Printf("create container success. pid: %d\n", initPid)
	} else {
		fmt.Printf("create container success. ID: %s\n", opt.ContainerId)
	}

	// 7. cgroup setup
	if err := c.containerCgroupPreparer.prepare(opt.ContainerId, spec, initPid); err != nil {
		return err
	}

	// 8. network setup
	if err := c.containerNetworkPreparer.prepare(opt.ContainerId, initPid, spec.Annotations); err != nil {
		return err
	}

	// 9. update state.json
	//      status = created
	//      pid    = init pid
	//		shimPid = 0
	if err := c.containerStatusManager.UpdateStatus(
		opt.ContainerId,
		status.CREATED,
		initPid,
		0,
	); err != nil {
		return err
	}

	// 10. HOOK: createContainer
	if err := c.containerHookController.RunCreateContainerHooks(
		opt.ContainerId,
		spec.Hooks.CreateContainer,
	); err != nil {
		return err
	}

	// 11. HOOK: startContainer
	if err := c.containerHookController.RunStartContainerHooks(
		opt.ContainerId,
		spec.Hooks.StartContainer,
	); err != nil {
		return err
	}

	// 12. start container
	if err := c.containerStart.Execute(
		StartOption{ContainerId: opt.ContainerId},
	); err != nil {
		return err
	}

	// 13. update state.json
	//       status = running
	if err := c.containerStatusManager.UpdateStatus(
		opt.ContainerId,
		status.RUNNING,
		-1, // no update
		-1, // no update
	); err != nil {
		return err
	}

	// 14. HOOK: poststart
	if err := c.containerHookController.RunPoststartHooks(
		opt.ContainerId,
		spec.Hooks.Poststart,
	); err != nil {
		return err
	}

	// 15. wait init process
	if opt.Tty {
		if err := cmd.Wait(); err != nil {
			return err
		}

		// 16. update state.json
		//        status = stopped
		if err := c.containerStatusManager.UpdateStatus(
			opt.ContainerId,
			status.STOPPED,
			0,
			0,
		); err != nil {
			return err
		}
	}

	return nil
}
