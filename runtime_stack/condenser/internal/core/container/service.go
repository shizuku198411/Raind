package container

import (
	"condenser/internal/core/image"
	"condenser/internal/core/network"
	"condenser/internal/runtime"
	"condenser/internal/runtime/droplet"
	"condenser/internal/store/csm"
	"condenser/internal/store/ilm"
	"condenser/internal/store/ipam"
	"condenser/internal/store/psm"
	"condenser/internal/utils"
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
