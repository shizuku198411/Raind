package container

import (
	"condenser/internal/runtime"
	"fmt"
)

// == service: start ==
func (s *ContainerService) Start(startParameter ServiceStartModel) (string, error) {
	// resolve container id
	containerId, err := s.csmHandler.ResolveContainerId(startParameter.ContainerId)
	if err != nil {
		return "", fmt.Errorf("container: %s not found", startParameter.ContainerId)
	}

	containerInfo, err := s.csmHandler.GetContainerById(containerId)
	if containerInfo.BottleId != "" && !startParameter.OpBottle {
		return "", fmt.Errorf(
			"direct operation of container: %s is not supported as it's managed by bottle: %s.\nuse 'raind bottle start <bottle-id|bottle-name>'",
			startParameter.ContainerId, containerInfo.BottleId,
		)
	}

	switch containerInfo.State {
	case "created":
		// start container
		if err := s.startContainer(containerId, startParameter.Tty); err != nil {
			return "", fmt.Errorf("start container failed: %w", err)
		}

	case "running":
		// already started. ignore operation
		return "", fmt.Errorf("container: %s already started", containerId)

	case "stopped":
		// get tty flag
		containerInfo, err := s.csmHandler.GetContainerById(containerId)
		if err != nil {
			return "", err
		}
		// create container
		if containerInfo.PodId != "" {
			if err := s.joinContainer(containerId, containerInfo.Tty, containerInfo.PodId); err != nil {
				return "", fmt.Errorf("start container failed: %w", err)
			}
		} else {
			if err := s.createContainer(containerId, containerInfo.Tty); err != nil {
				return "", fmt.Errorf("start container failed: %w", err)
			}
		}
		// start container
		if err := s.startContainer(containerId, containerInfo.Tty); err != nil {
			return "", fmt.Errorf("start container failed: %w", err)
		}

	default:
		return "", fmt.Errorf("start operation not allowed to current container status: %s", containerInfo.State)
	}

	return containerId, nil
}

func (s *ContainerService) startContainer(containerId string, tty bool) error {
	// runtime: start
	if err := s.runtimeHandler.Start(
		runtime.StartModel{
			ContainerId: containerId,
			Tty:         tty,
		},
	); err != nil {
		return err
	}

	return nil
}
