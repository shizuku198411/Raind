package pod

import (
	"condenser/internal/core/container"
	"condenser/internal/store/psm"
	"fmt"
)

// == service: start pod sandbox ==
func (s *PodService) Start(podId string) (string, error) {
	podInfo, err := s.psmHandler.GetPodById(podId)
	if err != nil {
		return "", err
	}
	if err := s.ensurePodTemplateConsistency(podInfo); err != nil {
		return "", err
	}
	if err := s.ensurePodTemplateContainers(podInfo); err != nil {
		return "", err
	}

	containers, err := s.containerHandler.GetContainersByPodId(podId)
	if err != nil {
		return "", err
	}

	for _, c := range containers {
		if s.isPodInfraName(c.Name) {
			if c.State == "running" {
				continue
			}
			if _, err := s.containerHandler.Start(container.ServiceStartModel{ContainerId: c.ContainerId, Tty: false}); err != nil {
				return "", err
			}
		}
	}

	for _, c := range containers {
		if s.isPodInfraName(c.Name) {
			continue
		}
		if c.State == "running" {
			continue
		}
		if _, err := s.containerHandler.Start(container.ServiceStartModel{ContainerId: c.ContainerId, Tty: false}); err != nil {
			return "", err
		}
	}

	if err := s.psmHandler.UpdatePod(podId, "running"); err != nil {
		return "", err
	}
	_ = s.psmHandler.UpdatePodStoppedByUser(podId, false)

	return podId, nil
}

func (s *PodService) ensurePodTemplateConsistency(podInfo psm.PodInfo) error {
	if podInfo.TemplateId == "" {
		return nil
	}
	tpl, err := s.psmHandler.GetPodTemplate(podInfo.TemplateId)
	if err != nil {
		return err
	}

	if tpl.Spec.Name != podInfo.Name || tpl.Spec.Namespace != podInfo.Namespace {
		inUse, err := s.psmHandler.IsTemplateReferenced(podInfo.TemplateId)
		if err != nil {
			return err
		}
		if !inUse {
			return fmt.Errorf("pod template mismatch: name/namespace")
		}
	}
	if !equalStringMap(tpl.Spec.Labels, podInfo.Labels) {
		return fmt.Errorf("pod template mismatch: labels")
	}
	if !equalStringMap(tpl.Spec.Annotations, podInfo.Annotations) {
		return fmt.Errorf("pod template mismatch: annotations")
	}
	if len(tpl.Spec.Containers) > 0 {
		for _, spec := range tpl.Spec.Containers {
			if spec.Name == "" {
				continue
			}
			if spec.Image == "" {
				return fmt.Errorf("pod template invalid: container image required: %s", spec.Name)
			}
		}
	}

	return nil
}

func equalStringMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func (s *PodService) ensurePodTemplateContainers(podInfo psm.PodInfo) error {
	if podInfo.TemplateId == "" {
		return nil
	}
	tpl, err := s.psmHandler.GetPodTemplate(podInfo.TemplateId)
	if err != nil {
		return err
	}
	if len(tpl.Spec.Containers) == 0 {
		return nil
	}

	containers, err := s.containerHandler.GetContainersByPodId(podInfo.PodId)
	if err != nil {
		return err
	}
	actualByName := make(map[string]container.ContainerState, len(containers))
	for _, c := range containers {
		if s.isPodInfraName(c.Name) {
			continue
		}
		actualByName[c.Name] = c
	}

	for _, spec := range tpl.Spec.Containers {
		if spec.Name == "" {
			continue
		}
		if spec.Image == "" {
			return fmt.Errorf("pod template invalid: container image required: %s", spec.Name)
		}
		expectedName := s.buildPodMemberName(spec.Name, podInfo.PodId)
		if _, ok := actualByName[expectedName]; ok {
			continue
		}
		if _, err := s.containerHandler.Create(container.ServiceCreateModel{
			Image:   spec.Image,
			Command: spec.Command,
			Port:    spec.Port,
			Mount:   spec.Mount,
			Env:     spec.Env,
			Network: spec.Network,
			Tty:     spec.Tty,
			Name:    spec.Name,
			PodId:   podInfo.PodId,
		}); err != nil {
			return err
		}
	}

	return nil
}

func (s *PodService) buildPodMemberName(baseName, podId string) string {
	if baseName == "" {
		return baseName
	}
	suffix := podId
	if len(suffix) > 8 {
		suffix = suffix[len(suffix)-8:]
	}
	return baseName + "-" + suffix
}
