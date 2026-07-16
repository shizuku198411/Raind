package kill

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"raind/internal/droplet/container/signals"
	"raind/internal/droplet/hook"
	"raind/internal/droplet/logs"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
)

type Option struct {
	ContainerId string
	Signal      string
}

type LoadSpecFunc func(containerId string) (spec.Spec, error)

type Controller struct {
	LoadSpec                LoadSpecFunc
	SyscallHandler          utils.KernelSyscallHandler
	ContainerStatusManager  status.ContainerStatusManager
	ContainerHookController hook.ContainerHookController
}

func (c *Controller) Kill(opt Option) (err error) {
	var (
		specFile spec.Spec
		event    = "kill"
		stage    string
		signal   []string
		pid      int
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
			Spec:        &specFile,
			Pid:         pid,
			Signals:     &signal,
			Result:      result,
			Error:       err,
		})
	}()

	stage = "load_spec"
	specFile, err = c.LoadSpec(opt.ContainerId)
	if err != nil {
		return err
	}

	stage = "get_status"
	containerStatus, err := c.ContainerStatusManager.GetStatusFromId(opt.ContainerId)
	if err != nil {
		return err
	}
	signalName, sig, err := signals.Parse(opt.Signal)
	if err != nil {
		return err
	}
	signal = append(signal, signalName)

	stage = "check_status"
	if containerStatus != status.RUNNING && containerStatus != status.CREATED {
		if containerStatus == status.STOPPED && signalName == "KILL" && HasExternalPidFileMarker(c.SyscallHandler, opt.ContainerId) {
			return nil
		}
		return fmt.Errorf("container: %s neither created nor running.", opt.ContainerId)
	}

	stage = "get_pid"
	containerPid, err := c.ContainerStatusManager.GetPidFromId(opt.ContainerId)
	if err != nil {
		return err
	}
	stage = "get_shim_pid"
	shimPid, err := c.ContainerStatusManager.GetShimPidFromId(opt.ContainerId)
	if err != nil {
		return err
	}

	stage = "send_signal"
	procStartTime, err := ReadProcStartTime(containerPid)
	if err != nil {
		return err
	}
	procIdentity := ProcIdentity{
		Pid:       containerPid,
		StartTime: procStartTime,
	}
	err = c.SyscallHandler.Kill(containerPid, sig)
	if err != nil {
		return err
	}
	if signalName == "TERM" {
		stage = "wait_exit_grace"
		err = WaitProcessExit(procIdentity, 3*time.Second)
		if err != nil {
			stage = "send_sigkill"
			_ = c.SyscallHandler.Kill(containerPid, signals.Map["KILL"])
			signal = append(signal, "KILL")

			stage = "wait_exit_kill"
			err = WaitProcessExit(procIdentity, 5*time.Second)
			if err != nil {
				return fmt.Errorf("failed to stop container pid=%d: %w", containerPid, err)
			}
		}
	}
	if signalName == "KILL" {
		stage = "wait_exit_signal"
		_ = WaitProcessExit(procIdentity, 5*time.Second)
	}
	stoppedBySignal := containerStatus == status.CREATED || signalName == "TERM" || signalName == "KILL"

	stage = "cleanup_shim"
	if stoppedBySignal && shimPid > 0 {
		_ = CleanupShim(c.SyscallHandler, opt.ContainerId)
	}

	stage = "update_state"
	nextStatus := status.STOPPED
	nextPid := 0
	nextShimPid := 0
	if !stoppedBySignal {
		nextStatus = status.RUNNING
		nextPid = -1
		nextShimPid = -1
	}
	err = c.ContainerStatusManager.UpdateStatus(opt.ContainerId, nextStatus, nextPid, nextShimPid)
	if err != nil {
		return err
	}

	if !stoppedBySignal {
		return nil
	}
	stage = "hook_stopContainer"
	err = c.ContainerHookController.RunStopContainerHooks(
		opt.ContainerId,
		specFile.Hooks.StopContainer,
	)
	if err != nil {
		return err
	}

	return nil
}

func HasExternalPidFileMarker(syscallHandler utils.KernelSyscallHandler, containerId string) bool {
	_, err := syscallHandler.Stat(utils.ExternalPidFileMarkerPath(containerId))
	return err == nil
}

func CleanupShim(syscallHandler utils.KernelSyscallHandler, containerId string) error {
	if err := syscallHandler.Remove(utils.SockPath(containerId)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := syscallHandler.Remove(utils.InitPidFilePath(containerId)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func WaitProcessExit(procIdentity ProcIdentity, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		currentStart, err := ReadProcStartTime(procIdentity.Pid)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
		}
		state, err := ReadProcState(procIdentity.Pid)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
		} else if state == "Z" {
			return nil
		}
		if currentStart != procIdentity.StartTime {
			return nil
		}
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

func ReadProcStartTime(pid int) (uint64, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return 0, err
	}
	fields, err := ParseProcStatFields(string(b))
	if err != nil {
		return 0, err
	}
	startTime, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0, err
	}
	return startTime, nil
}

func ReadProcState(pid int) (string, error) {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return "", err
	}
	fields, err := ParseProcStatFields(string(b))
	if err != nil {
		return "", err
	}
	if len(fields) == 0 {
		return "", fmt.Errorf("invalid stat format")
	}
	return fields[0], nil
}

func ParseProcStatFields(s string) ([]string, error) {
	idx := strings.LastIndex(s, ")")
	if idx < 0 {
		return nil, fmt.Errorf("invalid stat format")
	}
	fields := strings.Fields(s[idx+1:])
	if len(fields) < 20 {
		return nil, fmt.Errorf("invalid stat format")
	}
	return fields, nil
}
