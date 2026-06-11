package container

import (
	"raind/internal/condenser/core/image"
	"raind/internal/condenser/core/network"
	"raind/internal/condenser/runtime"
	"raind/internal/condenser/runtime/droplet"
	"raind/internal/condenser/store/csm"
	"raind/internal/condenser/store/ilm"
	"raind/internal/condenser/store/ipam"
	"raind/internal/condenser/store/psm"
	"raind/internal/condenser/utils"
)

func NewContaierService() *ContainerService {
	return &ContainerService{
		filesystemHandler: utils.NewFilesystemExecutor(),
		commandFactory:    utils.NewCommandFactory(),
		runtimeHandler:    droplet.NewDropletHandler(),

		ipamHandler: ipam.NewIpamManager(ipam.NewIpamStore(utils.IpamStorePath)),
		ilmHandler:  ilm.NewIlmManager(ilm.NewIlmStore(utils.IlmStorePath)),
		csmHandler:  csm.NewCsmManager(csm.NewCsmStore(utils.CsmStorePath)),
		psmHandler:  psm.NewPsmManager(psm.NewPsmStore(utils.PsmStorePath)),

		imageServiceHandler:   image.NewImageService(),
		networkServiceHandler: network.NewNetworkService(),
	}
}

type ContainerService struct {
	filesystemHandler utils.FilesystemHandler
	commandFactory    utils.CommandFactory
	runtimeHandler    runtime.RuntimeHandler

	ipamHandler ipam.IpamHandler
	ilmHandler  ilm.IlmHandler
	csmHandler  csm.CsmHandler
	psmHandler  psm.PsmHandler

	imageServiceHandler   image.ImageServiceHandler
	networkServiceHandler network.NetworkServiceHandler
}

func (s *ContainerService) getContainerState(containerId string) (string, error) {
	containerInfo, err := s.csmHandler.GetContainerById(containerId)
	if err != nil {
		return "", err
	}
	return containerInfo.State, nil
}
