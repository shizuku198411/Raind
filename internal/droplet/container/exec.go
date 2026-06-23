package container

import (
	"fmt"
	"os"
	"raind/internal/droplet/logs"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
	"slices"
	"strconv"
)

// NewContainerExec constructs a ContainerExec with the default
// implementations of its dependencies (CommandFactory, StatusManager).
// This acts as the main entry point for the `exec` workflow, which
// runs an additional process inside an existing container.
func NewContainerExec() *ContainerExec {
	return &ContainerExec{
		commandFactory:         utils.NewCommandFactory(),
		containerStatusManager: status.NewStatusHandler(),
		syscallHandler:         utils.NewSyscallHandler(),
		specLoader:             newFileSpecLoader(),
	}
}

// ContainerExec orchestrates execution of a new process inside an
// already-running container.
//
// It is responsible for:
//   - Verifying the container is in the RUNNING state
//   - Resolving the container’s init process PID
//   - Entering the container namespaces via nsenter
//   - Executing the requested command (optionally in interactive mode)
//
// Responsibility for low-level execution details is delegated to
// its collaborators to keep the workflow testable.
type ContainerExec struct {
	commandFactory         utils.CommandFactory
	containerStatusManager status.ContainerStatusManager
	syscallHandler         utils.KernelSyscallHandler
	specLoader             specLoader
}

// Exec runs the given entrypoint inside the target container.
//
// The workflow is:
//  1. Verify that the container is RUNNING
//  2. Look up the container’s PID from state.json
//  3. Construct an nsenter invocation targeting that PID and namespaces
//  4. Start the command
//  5. If interactive mode is enabled, attach stdio and wait for completion
//
// If any step fails, execution stops and the error is returned.
func (c *ContainerExec) Exec(opt ExecOption) (err error) {
	var (
		event = "exec"
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
			Command:     &opt.Entrypoint,
			Pid:         pid,
			Result:      result,
			Error:       err,
		})
	}()

	// 1. check container status
	//    if status is not running, return error
	stage = "get_status"
	containerStatus, err := c.containerStatusManager.GetStatusFromId(opt.ContainerId)
	if err != nil {
		return err
	}

	stage = "check_status"
	if containerStatus != status.RUNNING {
		return fmt.Errorf("container: %s not running.", opt.ContainerId)
	}

	// 2. retrieve pid from state.json
	stage = "get_pid"
	containerPid, err := c.containerStatusManager.GetPidFromId(opt.ContainerId)
	if err != nil {
		return err
	}

	stage = "load_spec"
	containerSpec, err := c.specLoader.loadFile(opt.ContainerId)
	if err != nil {
		return err
	}

	// 3. prepare entrypoint with nsenter
	if opt.Tty {
		stage = "exec_shim"
		err = c.executeShim(containerPid, opt)
		if err != nil {
			return err
		}
	} else {
		stage = "exec_nsenter"
		err = c.executeNsenter(containerPid, containerSpec, opt)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *ContainerExec) executeNsenter(containerPid int, containerSpec spec.Spec, opt ExecOption) error {
	entrypoint, err := resolveExecEntrypoint(strconv.Itoa(containerPid), containerSpec, opt.Entrypoint)
	if err != nil {
		return err
	}

	nsenterCommand := buildExecNsenterCommand(strconv.Itoa(containerPid), containerSpec)
	commandStr := slices.Concat(nsenterCommand, entrypoint)
	cmd := c.commandFactory.Command(commandStr[0], commandStr[1:]...)
	cmd.SetEnv(containerSpec.Process.Env)

	// set stdout/stderr to log files
	logPath := utils.ContainerLogPath(opt.ContainerId)
	f, err := c.syscallHandler.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return err
	}
	defer f.Close()
	cmd.SetStdout(f)
	cmd.SetStderr(f)

	// execute entrypoint and wait so callers can observe command failures.
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("exec command failed: %w: output=%q", err, readSmallLog(logPath))
	}

	return nil
}

func readSmallLog(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	const max = 4096
	if len(b) > max {
		b = b[len(b)-max:]
	}
	return string(b)
}

func (c *ContainerExec) executeShim(containerPid int, opt ExecOption) error {
	entrypoint := opt.Entrypoint
	shimArgs := append([]string{"exec-shim", opt.ContainerId, strconv.Itoa(containerPid)}, entrypoint...)
	cmd := c.commandFactory.Command(utils.SelfBinPath(), shimArgs...)

	// execute exec-shim subcommand
	if err := cmd.Start(); err != nil {
		return err
	}

	return nil
}
