package container

import (
	"time"

	networkpkg "raind/internal/droplet/container/network"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"
)

func newContainerNetworkController() *containerNetworkController {
	return &containerNetworkController{
		commandFactory:   &utils.ExecCommandFactory{},
		waitProcessNetns: waitProcessNetnsReady,
	}
}

type containerNetworkPreparer interface {
	prepare(containerId string, pid int, annotation spec.AnnotationObject) error
}

type containerNetworkController struct {
	commandFactory   utils.CommandFactory
	waitProcessNetns func(containerId string, pid int, timeout time.Duration, pollInterval time.Duration) error
}

func (c *containerNetworkController) prepare(containerId string, pid int, annotation spec.AnnotationObject) error {
	return c.controller().Prepare(containerId, pid, annotation)
}

func (c *containerNetworkController) controller() *networkpkg.Controller {
	waitProcessNetns := c.waitProcessNetns
	if waitProcessNetns == nil {
		waitProcessNetns = waitProcessNetnsReady
	}
	commandFactory := c.commandFactory
	if commandFactory == nil {
		commandFactory = &utils.ExecCommandFactory{}
	}
	return &networkpkg.Controller{
		CommandFactory:   commandFactory,
		WaitProcessNetns: waitProcessNetns,
	}
}

func waitProcessNetnsReady(containerId string, pid int, timeout time.Duration, pollInterval time.Duration) error {
	return networkpkg.WaitProcessNetnsReady(containerId, pid, timeout, pollInterval)
}

func readInitLogTail(containerId string, maxBytes int) string {
	return networkpkg.ReadInitLogTail(containerId, maxBytes)
}

func buildContainerVethPeerName(containerId string) string {
	return networkpkg.BuildContainerVethPeerName(containerId)
}

func buildProcessNetnsPath(pid int) string {
	return networkpkg.BuildProcessNetnsPath(pid)
}

func buildRootlessNetnsName(containerId string) string {
	return networkpkg.BuildRootlessNetnsName(containerId)
}

func buildNamedNetnsPath(netnsName string) string {
	return networkpkg.BuildNamedNetnsPath(netnsName)
}

func isRootlessAnnotation(annotation spec.AnnotationObject) bool {
	return networkpkg.IsRootlessAnnotation(annotation)
}
