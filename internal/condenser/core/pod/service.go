package pod

import (
	"raind/internal/condenser/core/container"
	"raind/internal/condenser/store/psm"
	"raind/internal/condenser/utils"
	"strings"
)

func NewPodService() *PodService {
	return &PodService{
		psmHandler:       psm.NewPsmManager(psm.NewPsmStore(utils.PsmStorePath)),
		containerHandler: container.NewContaierService(),
	}
}

type PodService struct {
	psmHandler       psm.PsmHandler
	containerHandler container.ContainerServiceHandler
}

func (s *PodService) isPodInfraName(name string) bool {
	return strings.HasPrefix(name, utils.PodInfraContainerNamePrefix)
}
