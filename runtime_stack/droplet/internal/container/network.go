package container

import (
	"droplet/internal/spec"
	"droplet/internal/utils"
	"fmt"
)

// newContainerNetworkController constructs a containerNetworkController with
// the default CommandFactory implementation. The controller is responsible
// for preparing container networking (veth creation and namespace setup)
// during container initialization.
func newContainerNetworkController() *containerNetworkController {
	return &containerNetworkController{
		commandFactory: &utils.ExecCommandFactory{},
	}
}

// containerNetworkPreparer defines the behavior required to prepare
// container networking resources for a target process. Implementations
// should configure interfaces according to annotation-provided settings.
type containerNetworkPreparer interface {
	prepare(containerId string, pid int, annotation spec.AnnotationObject) error
}

// containerNetworkController is the default implementation of
// containerNetworkPreparer. It sets up a veth pair, attaches it to the
// host bridge, and configures the container network namespace.
type containerNetworkController struct {
	commandFactory utils.CommandFactory
}

// prepare configures networking for the given container process.
//
// The workflow is:
//  1. Parse the network configuration from container annotations
//  2. Create and attach a veth pair on the host side
//  3. Enter the container network namespace and configure the interface
//
// Returns an error if any networking operation fails.
func (c *containerNetworkController) prepare(containerId string, pid int, annotation spec.AnnotationObject) error {
	// 1. retrieve network config from annotation
	var networkConfig spec.NetConfigObject
	if err := utils.StringToJson(annotation.Net, &networkConfig); err != nil {
		return err
	}

	if networkConfig.Interface.Name == "" || networkConfig.Interface.IPv4.Address == "" {
		return nil
	}

	// 2. create veth pair
	if err := c.createVethPair(containerId, pid, networkConfig); err != nil {
		return err
	}

	// 3. setup inside container
	if err := c.setupContainerNetns(pid, networkConfig); err != nil {
		return err
	}
	return nil
}

// createVethPair creates the veth pair used for container networking.
//
// Host-side operations performed:
//  1. Create a veth pair (host ↔ container)
//  2. Attach the host-side veth to the specified bridge
//  3. Bring the host-side veth interface up
func (c *containerNetworkController) createVethPair(containerId string, pid int, networkConfig spec.NetConfigObject) error {
	pidStr := fmt.Sprint(pid)

	// 1. create veth
	createVeth := c.commandFactory.Command("ip", "link", "add", "name", networkConfig.Interface.Name, "type", "veth", "peer", "name", networkConfig.HostInterface, "netns", pidStr)
	if err := createVeth.Run(); err != nil {
		return err
	}

	// 2. attach veth to bridge
	attacheVeth := c.commandFactory.Command("ip", "link", "set", networkConfig.Interface.Name, "master", networkConfig.BridgeInterface)
	if err := attacheVeth.Run(); err != nil {
		return err
	}

	// 3. up veth
	upVeth := c.commandFactory.Command("ip", "link", "set", networkConfig.Interface.Name, "up")
	if err := upVeth.Run(); err != nil {
		return err
	}
	return nil
}

// setupContainerNetns configures networking inside the container's
// network namespace.
//
// Inside-namespace operations performed:
//  1. Bring up loopback
//  2. Rename the veth interface
//  3. Assign the IPv4 address
//  4. Bring the interface up
//  5. Configure the default gateway
func (c *containerNetworkController) setupContainerNetns(pid int, networkConfig spec.NetConfigObject) error {
	pidStr := fmt.Sprint(pid)

	// 1. up loopback i/f
	upLoopbackIf := c.commandFactory.Command("nsenter", "-t", pidStr, "-n", "ip", "link", "set", "lo", "up")
	if err := upLoopbackIf.Run(); err != nil {
		return err
	}

	// 2. rename veth
	renameVeth := c.commandFactory.Command("nsenter", "-t", pidStr, "-n", "ip", "link", "set", networkConfig.HostInterface, "name", "eth0")
	if err := renameVeth.Run(); err != nil {
		return err
	}

	// 3. assign address
	assignAddr := c.commandFactory.Command("nsenter", "-t", pidStr, "-n", "ip", "addr", "add", networkConfig.Interface.IPv4.Address, "dev", "eth0")
	if err := assignAddr.Run(); err != nil {
		return err
	}

	// 4. up veth
	upVeth := c.commandFactory.Command("nsenter", "-t", pidStr, "-n", "ip", "link", "set", "eth0", "up")
	if err := upVeth.Run(); err != nil {
		return err
	}

	// 5. set gateway
	setGateway := c.commandFactory.Command("nsenter", "-t", pidStr, "-n", "ip", "route", "add", "default", "via", networkConfig.Interface.IPv4.Gateway)
	if err := setGateway.Run(); err != nil {
		return err
	}

	return nil
}
