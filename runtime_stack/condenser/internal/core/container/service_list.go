package container

import "condenser/internal/store/csm"

// == service: get container list ==
func (s *ContainerService) GetContainerList() ([]ContainerState, error) {
	containerList, err := s.csmHandler.GetContainerList()
	if err != nil {
		return nil, err
	}
	return s.buildContainerStateList(containerList)
}

// == service: get container list by pod ==
func (s *ContainerService) GetContainersByPodId(podId string) ([]ContainerState, error) {
	containerList, err := s.csmHandler.GetContainersByPodId(podId)
	if err != nil {
		return nil, err
	}
	return s.buildContainerStateList(containerList)
}

func (s *ContainerService) buildContainerStateList(containerList []csm.ContainerInfo) ([]ContainerState, error) {
	poolList, err := s.ipamHandler.GetPoolList()
	if err != nil {
		return nil, err
	}

	var containerStateList []ContainerState
	for _, c := range containerList {
		var (
			forwards []ForwardInfo
			address  string
		)
		for _, p := range poolList {
			for addr, info := range p.Allocations {
				if info.ContainerId != c.ContainerId {
					continue
				}
				address = addr
				for _, f := range info.Forwards {
					forwards = append(forwards, ForwardInfo{
						HostPort:      f.HostPort,
						ContainerPort: f.ContainerPort,
						Protocol:      f.Protocol,
					})
				}
			}
		}

		containerStateList = append(containerStateList, ContainerState{
			ContainerId: c.ContainerId,
			Name:        c.ContainerName,
			PodId:       c.PodId,
			State:       c.State,
			Pid:         c.Pid,
			Repository:  c.Repository,
			Reference:   c.Reference,
			Command:     c.Command,

			Address:  address,
			Forwards: forwards,

			CreatingAt: c.CreatingAt,
			CreatedAt:  c.CreatedAt,
			StartedAt:  c.StartedAt,
			StoppedAt:  c.StoppedAt,
		})
	}

	return containerStateList, nil
}

// =================================

// == service: get container by id ==
func (s *ContainerService) GetContainerById(containerId string) (ContainerState, error) {
	containerState, err := s.csmHandler.GetContainerById(containerId)
	if err != nil {
		return ContainerState{}, err
	}
	address, networkState, err := s.ipamHandler.GetNetworkInfoById(containerId)
	if err != nil {
		return ContainerState{}, err
	}

	var forwards []ForwardInfo
	for _, f := range networkState.Forwards {
		forwards = append(forwards, ForwardInfo{
			HostPort:      f.HostPort,
			ContainerPort: f.ContainerPort,
			Protocol:      f.Protocol,
		})
	}

	return ContainerState{
		ContainerId: containerState.ContainerId,
		PodId:       containerState.PodId,
		State:       containerState.State,
		Pid:         containerState.Pid,
		Repository:  containerState.Repository,
		Reference:   containerState.Reference,
		Command:     containerState.Command,

		Address:  address,
		Forwards: forwards,

		CreatingAt: containerState.CreatingAt,
		CreatedAt:  containerState.CreatedAt,
		StartedAt:  containerState.StartedAt,
		StoppedAt:  containerState.StoppedAt,
	}, nil
}
