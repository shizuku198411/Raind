package container

import "fmt"

func (s *ContainerService) InspectContainer(target string) (ContainerInspect, error) {
	containerId, err := s.csmHandler.ResolveContainerId(target)
	if err != nil {
		return ContainerInspect{}, fmt.Errorf("container: %s not found", target)
	}
	info, err := s.csmHandler.GetContainerById(containerId)
	if err != nil {
		return ContainerInspect{}, err
	}
	config, err := s.GetContainerSpec(containerId)
	if err != nil {
		return ContainerInspect{}, err
	}
	sanitizeInspectConfig(config)

	securityProfile := info.SecurityProfile
	if securityProfile == "" {
		securityProfile = "default"
	}

	return ContainerInspect{
		ContainerId:     info.ContainerId,
		Name:            info.ContainerName,
		PodId:           info.PodId,
		DropletId:       info.DropletId,
		State:           info.State,
		Pid:             info.Pid,
		ImageRepository: info.Repository,
		ImageReference:  info.Reference,
		Command:         info.Command,
		SecurityProfile: securityProfile,
		LogPath:         info.LogPath,
		Tty:             info.Tty,
		CreatedAt:       info.CreatedAt,
		StartedAt:       info.StartedAt,
		StoppedAt:       info.StoppedAt,
		Config:          config,
	}, nil
}

func sanitizeInspectConfig(config map[string]any) {
	process, _ := config["process"].(map[string]any)
	delete(process, "capabilities")

	linuxSpec, _ := config["linux"].(map[string]any)
	delete(linuxSpec, "seccomp")
	delete(linuxSpec, "apparmorProfile")
}
