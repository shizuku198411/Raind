package pod

import "condenser/internal/core/container"

// == service: stop pod sandbox ==
func (s *PodService) Stop(podId string) (string, error) {
	containers, err := s.containerHandler.GetContainersByPodId(podId)
	if err != nil {
		return "", err
	}
	for _, c := range containers {
		if s.isPodInfraName(c.Name) {
			continue
		}
		if _, err := s.containerHandler.Stop(container.ServiceStopModel{ContainerId: c.ContainerId}); err != nil {
			return "", err
		}
	}
	if err := s.psmHandler.UpdatePod(podId, "stopped"); err != nil {
		return "", err
	}
	if err := s.psmHandler.UpdatePodStoppedByUser(podId, true); err != nil {
		return "", err
	}
	return podId, nil
}
