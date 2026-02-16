package container

import (
	"context"
	"droplet/internal/hook"
	"droplet/internal/logs"
	"droplet/internal/spec"
	"droplet/internal/status"
	"droplet/internal/utils"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// NewContainerKill constructs a ContainerKill with the default
// implementations of its dependencies (SyscallHandler, StatusManager).
// This serves as the main entry point for the `kill` workflow, which
// delivers a signal to a running container’s init process.
func NewContainerKill() *ContainerKill {
	return &ContainerKill{
		specLoader:              newFileSpecLoader(),
		syscallHandler:          utils.NewSyscallHandler(),
		containerStatusManager:  status.NewStatusHandler(),
		containerHookController: hook.NewHookController(),
	}
}

// ContainerKill orchestrates the container termination flow.
//
// It is responsible for:
//   - Verifying that the container is currently RUNNING
//   - Resolving the container’s init process PID from state.json
//   - Sending the requested signal to that process
//   - Updating the container status to STOPPED
//
// Low-level system interactions are delegated to collaborators to
// keep the workflow testable and replaceable.
type ContainerKill struct {
	specLoader              specLoader
	syscallHandler          utils.KernelSyscallHandler
	containerStatusManager  status.ContainerStatusManager
	containerHookController hook.ContainerHookController
}

// Kill sends a signal to the container’s init process and updates its state.
//
// The workflow is:
//  1. Check that the container is RUNNING
//  2. Retrieve the init PID from state.json
//  3. Send the configured signal to that PID
//  4. Update the status file to STOPPED and clear the PID
//
// If any step fails, the method stops and returns the error.
func (c *ContainerKill) Kill(opt KillOption) (err error) {
	var (
		spec   spec.Spec
		event  = "kill"
		stage  string
		signal []string
		pid    int
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
			Spec:        &spec,
			Pid:         pid,
			Signals:     &signal,
			Result:      result,
			Error:       err,
		})
	}()

	// 1. load config.json
	stage = "load_spec"
	spec, err = c.specLoader.loadFile(opt.ContainerId)
	if err != nil {
		return err
	}

	// 2. check container status
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

	// 3. retrieve pid and shimpid from state.json
	stage = "get_pid"
	containerPid, err := c.containerStatusManager.GetPidFromId(opt.ContainerId)
	if err != nil {
		return err
	}
	stage = "get_shim_pid"
	shimPid, err := c.containerStatusManager.GetShimPidFromId(opt.ContainerId)
	if err != nil {
		return err
	}

	// 4. send signal to pid
	stage = "send_signal"
	procStartTime, err := c.readProcStartTime(containerPid)
	if err != nil {
		return err
	}
	procIdentity := ProcIdentity{
		Pid:       containerPid,
		StartTime: procStartTime,
	}
	err = c.syscallHandler.Kill(containerPid, signalMap[opt.Signal])
	signal = append(signal, opt.Signal)
	if err != nil {
		return err
	}
	// if signal is SIGTERM, graceful stop with SIGKILL
	if opt.Signal == "TERM" {
		stage = "wait_exit_grace"
		err = c.waitProcessExit(procIdentity, 3*time.Second)
		if err != nil {
			// timeout: send SIGKILL
			stage = "send_sigkill"
			_ = c.syscallHandler.Kill(containerPid, signalMap["KILL"])
			signal = append(signal, "KILL")

			stage = "wait_exit_kill"
			err = c.waitProcessExit(procIdentity, 5*time.Second)
			if err != nil {
				return fmt.Errorf("failed to stop container pid=%d: %w", containerPid, err)
			}
		}
	}

	// if shim pid > 0, the container created with interactive mode
	// clean up files for shim
	stage = "cleanup_shim"
	if shimPid > 0 {
		_ = c.cleanupShim(opt.ContainerId)
	}

	// 4. update status file
	//      status = stopped
	//      pid = 0
	//		shimPid = 0
	stage = "update_state"
	err = c.containerStatusManager.UpdateStatus(
		opt.ContainerId,
		status.STOPPED,
		0,
		0,
	)
	if err != nil {
		return err
	}

	// 5. HOOK: stopContainer
	stage = "hook_stopContainer"
	err = c.containerHookController.RunStopContainerHooks(
		opt.ContainerId,
		spec.Hooks.StopContainer,
	)
	if err != nil {
		return err
	}

	return nil
}

func (c *ContainerKill) cleanupShim(containerId string) error {
	// remove tty.sock
	if err := c.syscallHandler.Remove(utils.SockPath(containerId)); err != nil {
		return err
	}
	// remove init.pid
	if err := c.syscallHandler.Remove(utils.InitPidFilePath(containerId)); err != nil {
		return err
	}
	return nil
}

func (c *ContainerKill) waitProcessExit(procIdentity ProcIdentity, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		// retrieve proc starttime
		currentStart, err := c.readProcStartTime(procIdentity.Pid)
		if err != nil {
			if os.IsNotExist(err) {
				// exit
				return nil
			}
		}
		if currentStart != procIdentity.StartTime {
			// re-used proc, exit
			return nil
		}
		// exit when /proc/<pid> is removed
		if _, err := os.Stat(fmt.Sprintf("/proc/%d", procIdentity.Pid)); os.IsNotExist(err) {
			return nil
		}

		if time.Now().After(deadline) {
			return context.DeadlineExceeded
		}
		time.Sleep(50 * time.Millisecond)
	}
}

type ProcIdentity struct {
	Pid       int
	StartTime uint64
}

func (c *ContainerKill) readProcStartTime(pid int) (uint64, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}

	// stat format
	//  no  value     field
	// ---+---------+-----------
	//  1   12345     pid
	//  2   (bash)    command
	//  3   S         state
	//  4   652001    ppid
	//  5   652095    pgrp
	//  6   652095    session
	//  7   34819     tty_nr
	//  8   679797    tpgid
	//  9   4194304   flags
	//  10  220000    minflt
	//  11  1319082   cminflt
	//  12  0         majflt
	//  13  342       cmajflt
	//  14  175       utime
	//  15  162       stime
	//  16  2688      cutime
	//  17  1141      cstime
	//  18  20        priority
	//  19  0         nice
	//  20  1         num_threads
	//  21  0         itrealvalue
	//  22  48825543  **starttime**
	//  23  6381568   vsize
	//  24  1280      rss
	//       :
	s := string(b)
	idx := strings.LastIndex(s, ")")
	if idx < 0 {
		return 0, fmt.Errorf("invalid stat format")
	}
	fields := strings.Fields(s[idx+1:])
	startTime, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0, err
	}
	return startTime, nil
}
