package container

import (
	"condenser/internal/runtime"
	"condenser/internal/utils"
	"fmt"
	"strings"
)

// == service: stop ==
func (s *ContainerService) Stop(stopParameter ServiceStopModel) (string, error) {
	// resolve container id
	containerId, err := s.csmHandler.ResolveContainerId(stopParameter.ContainerId)
	if err != nil {
		return "", fmt.Errorf("container: %s not found", stopParameter.ContainerId)
	}

	containerInfo, err := s.csmHandler.GetContainerById(containerId)
	if containerInfo.BottleId != "" && !stopParameter.OpBottle {
		return "", fmt.Errorf(
			"direct operation of container: %s is not supported as it's managed by bottle: %s.\nuse 'raind bottle stop <bottle-id|bottle-name>'",
			stopParameter.ContainerId, containerInfo.BottleId,
		)
	}

	switch containerInfo.State {
	case "running":
		// stop container
		if err := s.stopContainer(containerId); err != nil {
			return "", fmt.Errorf("stop failed: %w", err)
		}
		if containerInfo.PodId != "" && !strings.HasPrefix(containerInfo.ContainerName, utils.PodInfraContainerNamePrefix) {
			_ = s.psmHandler.UpdatePod(containerInfo.PodId, "degraded")
		}
	default:
		return "", fmt.Errorf("stop operation not allowed to current container status: %s", containerInfo.State)
	}
	return containerId, nil
}

func (s *ContainerService) stopContainer(containerId string) error {
	// runtime: stop
	if err := s.runtimeHandler.Stop(
		runtime.StopModel{
			ContainerId: containerId,
		},
	); err != nil {
		return err
	}
	return nil
}
