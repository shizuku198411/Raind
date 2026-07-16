package statusflow

import (
	"fmt"

	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
)

func Current(manager status.ContainerStatusManager, containerId string) (status.ContainerStatus, error) {
	return manager.GetStatusFromId(containerId)
}

func RequireCreated(manager status.ContainerStatusManager, containerId string) (status.ContainerStatus, error) {
	current, err := Current(manager, containerId)
	if err != nil {
		return current, err
	}
	return current, EnsureCreated(containerId, current)
}

func EnsureCreated(containerId string, current status.ContainerStatus) error {
	if current != status.CREATED {
		return fmt.Errorf("container: %s is not created. currnet status: %s", containerId, current)
	}
	return nil
}

func RequireRunning(manager status.ContainerStatusManager, containerId string) (status.ContainerStatus, error) {
	current, err := Current(manager, containerId)
	if err != nil {
		return current, err
	}
	return current, EnsureRunning(containerId, current)
}

func EnsureRunning(containerId string, current status.ContainerStatus) error {
	if current != status.RUNNING {
		return fmt.Errorf("container: %s not running.", containerId)
	}
	return nil
}

func IsAllowed(current status.ContainerStatus, allowed ...status.ContainerStatus) bool {
	for _, item := range allowed {
		if current == item {
			return true
		}
	}
	return false
}

func Transition(manager status.ContainerStatusManager, containerId string, next status.ContainerStatus, pid int, shimPid int) error {
	return manager.UpdateStatus(containerId, next, pid, shimPid)
}

func Create(manager status.ContainerStatusManager, containerId string, pid int, next status.ContainerStatus, rootfs string, bundle string, annotation spec.AnnotationObject) error {
	return manager.CreateStatusFile(containerId, pid, next, rootfs, bundle, annotation)
}

func Remove(manager status.ContainerStatusManager, containerId string) error {
	return manager.RemoveStatusFile(containerId)
}

func CanDelete(current status.ContainerStatus, force bool) bool {
	return current == status.STOPPED || force
}

func ShouldKillBeforeDelete(current status.ContainerStatus, force bool) bool {
	return current == status.CREATED || (current == status.RUNNING && force)
}

func KillTransition(current status.ContainerStatus, signalName string) (next status.ContainerStatus, pid int, shimPid int, stoppedBySignal bool) {
	stoppedBySignal = current == status.CREATED || signalName == "TERM" || signalName == "KILL"
	if stoppedBySignal {
		return status.STOPPED, 0, 0, true
	}
	return status.RUNNING, -1, -1, false
}
