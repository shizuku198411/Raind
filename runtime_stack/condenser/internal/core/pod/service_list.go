package pod

// == service: list pods ==
func (s *PodService) GetPodList() ([]PodState, error) {
	podList, err := s.psmHandler.GetPodList()
	if err != nil {
		return nil, err
	}
	var result []PodState
	for _, p := range podList {
		desired, running, err := s.getContainerCounts(p.PodId, p.TemplateId)
		if err != nil {
			return nil, err
		}
		result = append(result, PodState{
			PodId:             p.PodId,
			Name:              p.Name,
			Namespace:         p.Namespace,
			UID:               p.UID,
			State:             p.State,
			DesiredContainers: desired,
			RunningContainers: running,
			NetworkNS:         p.NetworkNS,
			IPCNS:             p.IPCNS,
			UTSNS:             p.UTSNS,
			UserNS:            p.UserNS,
			Labels:            p.Labels,
			Annotations:       p.Annotations,
			CreatedAt:         p.CreatedAt,
			StartedAt:         p.StartedAt,
			StoppedAt:         p.StoppedAt,
		})
	}
	return result, nil
}

// == service: get pod by id ==
func (s *PodService) GetPodById(podId string) (PodState, error) {
	p, err := s.psmHandler.GetPodById(podId)
	if err != nil {
		return PodState{}, err
	}
	desired, running, err := s.getContainerCounts(p.PodId, p.TemplateId)
	if err != nil {
		return PodState{}, err
	}
	return PodState{
		PodId:             p.PodId,
		Name:              p.Name,
		Namespace:         p.Namespace,
		UID:               p.UID,
		State:             p.State,
		DesiredContainers: desired,
		RunningContainers: running,
		NetworkNS:         p.NetworkNS,
		IPCNS:             p.IPCNS,
		UTSNS:             p.UTSNS,
		UserNS:            p.UserNS,
		Labels:            p.Labels,
		Annotations:       p.Annotations,
		CreatedAt:         p.CreatedAt,
		StartedAt:         p.StartedAt,
		StoppedAt:         p.StoppedAt,
	}, nil
}

func (s *PodService) getContainerCounts(podId, templateId string) (int, int, error) {
	desired := 0
	if templateId != "" {
		if tmpl, err := s.psmHandler.GetPodTemplate(templateId); err == nil {
			desired = len(tmpl.Spec.Containers)
		}
	}
	containers, err := s.containerHandler.GetContainersByPodId(podId)
	if err != nil {
		return desired, 0, err
	}
	running := 0
	nonInfra := 0
	for _, c := range containers {
		if s.isPodInfraName(c.Name) {
			continue
		}
		nonInfra++
		if c.State == "running" {
			running++
		}
	}
	if desired == 0 {
		desired = nonInfra
	}
	return desired, running, nil
}
