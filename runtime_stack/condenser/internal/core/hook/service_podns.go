package hook

import (
	"condenser/internal/utils"
	"fmt"
	"strings"
)

func (s *HookService) updatePodNamespacesIfOwner(containerId string) error {
	containerInfo, err := s.csmHandler.GetContainerById(containerId)
	if err != nil {
		return err
	}
	if containerInfo.PodId == "" {
		return nil
	}
	if !strings.HasPrefix(containerInfo.ContainerName, utils.PodInfraContainerNamePrefix) {
		return nil
	}
	if containerInfo.Pid <= 0 {
		return nil
	}

	netNS := fmt.Sprintf("/proc/%d/ns/net", containerInfo.Pid)
	ipcNS := fmt.Sprintf("/proc/%d/ns/ipc", containerInfo.Pid)
	utsNS := fmt.Sprintf("/proc/%d/ns/uts", containerInfo.Pid)
	userNS := fmt.Sprintf("/proc/%d/ns/user", containerInfo.Pid)

	return s.psmHandler.UpdatePodNamespaces(containerInfo.Pid, containerInfo.PodId, netNS, ipcNS, utsNS, userNS)
}
