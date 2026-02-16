package container

import (
	"condenser/internal/core/network"
	"condenser/internal/runtime"
	"condenser/internal/utils"
	"fmt"
	"path/filepath"
	"strconv"
)

// == service: delete ==
func (s *ContainerService) Delete(deleteParameter ServiceDeleteModel) (string, error) {
	// resolve container id
	containerId, err := s.csmHandler.ResolveContainerId(deleteParameter.ContainerId)
	if err != nil {
		return "", fmt.Errorf("container: %s not found", deleteParameter.ContainerId)
	}

	containerInfo, err := s.csmHandler.GetContainerById(containerId)
	if containerInfo.BottleId != "" && !deleteParameter.OpBottle {
		return "", fmt.Errorf(
			"direct delete operation of container: %s is not supported as it's managed by bottle: %s.\nuse 'raind bottle rm <bottle-id|bottle-name>'",
			deleteParameter.ContainerId, containerInfo.BottleId,
		)
	}

	switch containerInfo.State {
	case "creating", "created", "stopped":
		// 1. delete container
		if err := s.deleteContainer(containerId); err != nil {
			return "", fmt.Errorf("delete container failed: %w", err)
		}

		// 2. cleanup forward rule
		if err := s.cleanupForwardRules(containerId); err != nil {
			return "", fmt.Errorf("cleanup forward rule failed: %w", err)
		}

		// 2. release address
		if err := s.releaseAddress(containerId); err != nil {
			return "", fmt.Errorf("release address failed: %w", err)
		}

		// 3. delete container directory
		if err := s.deleteContainerDirectory(containerId); err != nil {
			return "", fmt.Errorf("delete container directory failed: %w", err)
		}

		// 4. delete cgroup subtree
		if err := s.deleteCgroupSubtree(containerId); err != nil {
			return "", fmt.Errorf("delete cgroup subtree failed: %w", err)
		}
	default:
		return "", fmt.Errorf("delete operation not allowed to current container status: %s", containerInfo.State)
	}

	return containerId, nil
}

func (s *ContainerService) deleteContainer(containerId string) error {
	// runtime: delete
	if err := s.runtimeHandler.Delete(
		runtime.DeleteModel{
			ContainerId: containerId,
		},
	); err != nil {
		return err
	}
	return nil
}

func (s *ContainerService) cleanupForwardRules(containerId string) error {
	// retrieve network info
	forwards, err := s.ipamHandler.GetForwardInfo(containerId)
	if err != nil {
		return err
	}
	if len(forwards) == 0 {
		return nil
	}

	// remove rules
	for _, f := range forwards {
		if err := s.networkServiceHandler.RemoveForwardingRule(
			containerId,
			network.ServiceNetworkModel{
				HostPort:      strconv.Itoa(f.HostPort),
				ContainerPort: strconv.Itoa(f.ContainerPort),
				Protocol:      f.Protocol,
			},
		); err != nil {
			return err
		}
	}

	return nil
}

func (s *ContainerService) releaseAddress(containerId string) error {
	if err := s.ipamHandler.Release(containerId); err != nil {
		return err
	}
	return nil
}

func (s *ContainerService) deleteContainerDirectory(containerId string) error {
	containerDir := filepath.Join(utils.ContainerRootDir, containerId)
	if err := s.filesystemHandler.RemoveAll(containerDir); err != nil {
		return err
	}
	return nil
}

func (s *ContainerService) deleteCgroupSubtree(containerId string) error {
	cgroupPath := filepath.Join(utils.CgroupRuntimeDir, containerId)
	if err := s.filesystemHandler.Remove(cgroupPath); err != nil {
		return err
	}
	return nil
}
