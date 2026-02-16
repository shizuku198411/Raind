package pod

import (
	"condenser/internal/core/container"
	"condenser/internal/store/psm"
	"condenser/internal/utils"
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
