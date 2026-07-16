package pod

import (
	"errors"
	"os"
	"raind/internal/condenser/core/container"
	"strings"
)

// == service: remove pod sandbox ==
func (s *PodService) Remove(podId string) (string, error) {
	podInfo, err := s.psmHandler.GetPodById(podId)
	if err != nil {
		if isNotFoundErr(err) {
			return podId, nil
		}
		return "", err
	}
	if err := s.psmHandler.UpdatePodStoppedByUser(podId, true); err != nil {
		if isNotFoundErr(err) {
			return podId, nil
		}
		return "", err
	}

	containers, err := s.containerHandler.GetContainersByPodId(podId)
	if err != nil {
		if isNotFoundErr(err) {
			_ = s.psmHandler.RemovePod(podId)
			return podId, nil
		}
		return "", err
	}
	var infra container.ContainerState
	var hasInfra bool
	for _, c := range containers {
		if s.isPodInfraName(c.Name) {
			infra = c
			hasInfra = true
			continue
		}
		if err := s.stopContainerIgnoreStopped(c.ContainerId); err != nil {
			return "", err
		}
		if _, err := s.containerHandler.Delete(container.ServiceDeleteModel{ContainerId: c.ContainerId}); err != nil {
			if isNotFoundErr(err) {
				continue
			}
			return "", err
		}
	}
	if hasInfra {
		if err := s.stopContainerIgnoreStopped(infra.ContainerId); err != nil {
			return "", err
		}
		if _, err := s.containerHandler.Delete(container.ServiceDeleteModel{ContainerId: infra.ContainerId}); err != nil {
			if !isNotFoundErr(err) {
				return "", err
			}
		}
	}
	if err := s.psmHandler.RemovePod(podId); err != nil {
		if isNotFoundErr(err) {
			return podId, nil
		}
		return "", err
	}
	if podInfo.TemplateId != "" {
		inUse, err := s.psmHandler.IsTemplateReferenced(podInfo.TemplateId)
		if err == nil && !inUse {
			_ = s.psmHandler.RemovePodTemplate(podInfo.TemplateId)
		}
	}
	return podId, nil
}

func (s *PodService) stopContainerIgnoreStopped(containerId string) error {
	_, err := s.containerHandler.Stop(container.ServiceStopModel{ContainerId: containerId})
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "stop operation not allowed to current container status") {
		return nil
	}
	if isNotFoundErr(err) {
		return nil
	}
	return err
}

func isNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found") || strings.Contains(msg, "no such file")
}
