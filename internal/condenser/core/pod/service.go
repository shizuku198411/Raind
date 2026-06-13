package pod

import (
	"raind/internal/condenser/core/container"
	corenamespace "raind/internal/condenser/core/namespace"
	"raind/internal/condenser/store/psm"
	"raind/internal/condenser/utils"
	"strings"
)

func NewPodService() *PodService {
	return &PodService{
		psmHandler:       psm.NewPsmManager(psm.NewPsmStore(utils.PsmStorePath)),
		containerHandler: container.NewContaierService(),
		namespaceHandler: corenamespace.NewNamespaceService(),
	}
}

type PodService struct {
	psmHandler       psm.PsmHandler
	containerHandler container.ContainerServiceHandler
	namespaceHandler corenamespace.NamespaceServiceHandler
}

func (s *PodService) isPodInfraName(name string) bool {
	return strings.HasPrefix(name, utils.PodInfraContainerNamePrefix)
}

func (s *PodService) resolveContainerNetworks(namespace string, containers []psm.ContainerTemplateSpec) ([]psm.ContainerTemplateSpec, error) {
	if len(containers) == 0 {
		return containers, nil
	}
	networkName, err := s.namespaceHandler.ResolveNetwork(namespace)
	if err != nil {
		return nil, err
	}
	out := make([]psm.ContainerTemplateSpec, len(containers))
	copy(out, containers)
	for i := range out {
		if out[i].Network == "" {
			out[i].Network = networkName
		}
	}
	return out, nil
}
