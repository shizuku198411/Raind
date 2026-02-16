package status

import (
	"droplet/internal/oci"
	"droplet/internal/spec"
	"droplet/internal/utils"
	"os"
	"syscall"
)

// ContainerStatusManager defines the operations required to manage
// container state metadata (state.json).
//
// Implementations are responsible for creating, updating, reading,
// and deleting the status file, as well as querying PID and status
// for a given container ID.
type ContainerStatusManager interface {
	CreateStatusFile(containerId string, pid int, status ContainerStatus, rootfs string, bundle string, annotation spec.AnnotationObject) error
	RemoveStatusFile(containerId string) error
	ReadStatusFile(containerId string) (string, error)
	UpdateStatus(containerId string, status ContainerStatus, pid int, shimPid int) error
	UpdateExitCode(containerId string, exitCode int) error
	UpdateReasonAndMessage(containerId string, reasone string, message string) error
	GetPidFromId(containerId string) (int, error)
	GetStatusFromId(containerId string) (ContainerStatus, error)
	GetShimPidFromId(containerId string) (int, error)
	ListContainers() ([]StatusObject, error)
}

// NewStatusHandler constructs a StatusHandler with the default
// KernelSyscallHandler implementation. This is the default
// implementation of ContainerStatusManager used by the runtime.
func NewStatusHandler() *StatusHandler {
	return &StatusHandler{
		syscallHandler: utils.NewSyscallHandler(),
	}
}

// StatusHandler manages the lifecycle of container status files.
//
// It is responsible for:
//   - Creating state.json when a container is created
//   - Updating status and PID fields
//   - Deleting state.json when a container is removed
//   - Recomputing status based on the liveness of the container process
type StatusHandler struct {
	syscallHandler utils.KernelSyscallHandler
}

// CreateStatusFile creates or overwrites the status file (state.json)
// for the given container ID.
//
// It populates the file with the provided PID, status, rootfs, bundle
// path and annotations, along with the current OCI version.
func (h *StatusHandler) CreateStatusFile(containerId string, pid int, status ContainerStatus,
	rootfs string, bundle string, annotation spec.AnnotationObject) error {
	stateFilePath := utils.ContainerStatePath(containerId)
	statusObject := StatusObject{
		OciVersion: oci.OCIVersion,
		Id:         containerId,
		Status:     status.String(),
		Pid:        pid,
		ExitCode:   0,
		Reason:     "",
		Message:    "",
		ShimPid:    0,
		Rootfs:     rootfs,
		Bundle:     bundle,
		Annotaion:  annotation,
	}

	if err := utils.WriteJsonToFile(stateFilePath, statusObject); err != nil {
		return err
	}

	return nil
}

// RemoveStatusFile deletes the status file (state.json) associated
// with the given container ID.
func (h *StatusHandler) RemoveStatusFile(containerId string) error {
	stateFilePath := utils.ContainerStatePath(containerId)
	if err := h.syscallHandler.Remove(stateFilePath); err != nil {
		return err
	}
	return nil
}

// ReadStatusFile returns the raw JSON string from the status file
// for the given container ID.
//
// Before returning, it recomputes the status based on the current
// process liveness (e.g., updates RUNNING to STOPPED if the PID
// no longer exists), ensuring the file contents are up to date.
func (h *StatusHandler) ReadStatusFile(containerId string) (string, error) {
	stateFilePath := utils.ContainerStatePath(containerId)
	// load status file
	var statusObject StatusObject
	if err := utils.ReadJsonFile(stateFilePath, &statusObject); err != nil {
		return "", err
	}
	pid := statusObject.Pid
	currentStatus, parseErr := ParseContainerStatus(statusObject.Status)
	if parseErr != nil {
		return "", parseErr
	}

	// recompute status
	if err := h.recomputeStatus(containerId, pid, currentStatus); err != nil {
		return "", err
	}

	// read file
	data, err := os.ReadFile(stateFilePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// UpdateStatus updates the status and/or PID fields in the status file
// for the given container ID.
//
// If status is in the valid range, it is written. If pid is non-negative,
// it replaces the existing PID.
func (h *StatusHandler) UpdateStatus(containerId string, status ContainerStatus, pid int, shimPid int) error {
	stateFilePath := utils.ContainerStatePath(containerId)
	// load status file
	var statusObject StatusObject
	if err := utils.ReadJsonFile(stateFilePath, &statusObject); err != nil {
		return err
	}

	// update
	if status >= 0 && status <= 3 {
		statusObject.Status = status.String()
	}
	if pid >= 0 {
		statusObject.Pid = pid
	}
	if shimPid >= 0 {
		statusObject.ShimPid = shimPid
	}

	// write status file
	if err := utils.WriteJsonToFile(stateFilePath, statusObject); err != nil {
		return err
	}

	return nil
}

func (h *StatusHandler) UpdateExitCode(containerId string, exitCode int) error {
	stateFilePath := utils.ContainerStatePath(containerId)
	// load status file
	var statusObject StatusObject
	if err := utils.ReadJsonFile(stateFilePath, &statusObject); err != nil {
		return err
	}

	statusObject.ExitCode = exitCode

	// write status file
	if err := utils.WriteJsonToFile(stateFilePath, statusObject); err != nil {
		return err
	}

	return nil
}

func (h *StatusHandler) UpdateReasonAndMessage(containerId string, reasone string, message string) error {
	stateFilePath := utils.ContainerStatePath(containerId)
	// load status file
	var statusObject StatusObject
	if err := utils.ReadJsonFile(stateFilePath, &statusObject); err != nil {
		return err
	}

	statusObject.Reason = reasone
	statusObject.Message = message

	// write status file
	if err := utils.WriteJsonToFile(stateFilePath, statusObject); err != nil {
		return err
	}
	return nil
}

// GetPidFromId returns the PID recorded in the status file for the
// given container ID without recomputing the status.
func (h *StatusHandler) GetPidFromId(containerId string) (int, error) {
	stateFilePath := utils.ContainerStatePath(containerId)
	// load status file
	var statusObject StatusObject
	if err := utils.ReadJsonFile(stateFilePath, &statusObject); err != nil {
		return -1, err
	}
	return statusObject.Pid, nil
}

func (h *StatusHandler) GetShimPidFromId(containerId string) (int, error) {
	stateFilePath := utils.ContainerStatePath(containerId)
	// load status file
	var statusObject StatusObject
	if err := utils.ReadJsonFile(stateFilePath, &statusObject); err != nil {
		return -1, err
	}
	return statusObject.ShimPid, nil
}

// GetStatusFromId returns the current ContainerStatus for the given
// container ID.
//
// The status may be recomputed based on process liveness (for example,
// converting RUNNING to STOPPED if the recorded PID is no longer alive)
// before being returned.
func (h *StatusHandler) GetStatusFromId(containerId string) (ContainerStatus, error) {
	stateFilePath := utils.ContainerStatePath(containerId)
	// load status file
	var statusObject StatusObject
	if err := utils.ReadJsonFile(stateFilePath, &statusObject); err != nil {
		return -1, err
	}
	pid := statusObject.Pid
	currentStatus, parseErr := ParseContainerStatus(statusObject.Status)
	if parseErr != nil {
		return -1, parseErr
	}

	// recompute status
	if err := h.recomputeStatus(containerId, pid, currentStatus); err != nil {
		return -1, err
	}

	if err := utils.ReadJsonFile(stateFilePath, &statusObject); err != nil {
		return -1, err
	}
	newStatus, newParseErr := ParseContainerStatus(statusObject.Status)
	if newParseErr != nil {
		return -1, newParseErr
	}

	return newStatus, nil
}

// recomputeStatus recomputes and updates the status in the status file
// based on the liveness of the recorded PID.
//
// Currently, if the status is RUNNING but the process is no longer
// alive, it updates the status to STOPPED and clears the PID.
func (h *StatusHandler) recomputeStatus(containerId string, pid int, currentStatus ContainerStatus) error {
	if currentStatus == RUNNING {
		alive, _ := h.pidAlive(pid)
		if !alive {
			if err := h.UpdateStatus(containerId, STOPPED, 0, 0); err != nil {
				return err
			}
		}
	}
	return nil
}

// pidAlive reports whether a process with the given PID appears to be alive.
//
// It sends signal 0 to the PID:
//   - nil        => process exists and is accessible
//   - ESRCH      => process does not exist
//   - EPERM      => process exists but cannot be signaled due to permissions
func (h *StatusHandler) pidAlive(pid int) (bool, error) {
	if pid <= 0 {
		// process not exist
		return false, nil
	}

	// send 0 signal to process
	err := h.syscallHandler.Kill(pid, 0)
	if err == nil {
		// process exist
		return true, nil
	}
	if err == syscall.ESRCH {
		// no such process
		return false, nil
	}
	if err == syscall.EPERM {
		// operation not permitted, but process exist
		return true, nil
	}

	return false, nil
}

func (h *StatusHandler) ListContainers() ([]StatusObject, error) {
	var list []StatusObject

	containerBaseDir := utils.DefaultRootDir()

	entries, err := h.syscallHandler.ReadDir(containerBaseDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		containerId := entry.Name()
		stateFilePath := utils.ContainerStatePath(containerId)
		// load status file
		var statusObject StatusObject
		if err := utils.ReadJsonFile(stateFilePath, &statusObject); err != nil {
			// skip if state.json is not exist
			continue
		}

		// recompute status
		currentStatus, err := ParseContainerStatus(statusObject.Status)
		if err != nil {
			return nil, err
		}
		if err := h.recomputeStatus(statusObject.Id, statusObject.Pid, currentStatus); err != nil {
			return nil, err
		}

		// reload status file
		if err := utils.ReadJsonFile(stateFilePath, &statusObject); err != nil {
			// skip if state.json is not exist
			continue
		}

		list = append(list, statusObject)
	}

	return list, nil
}
