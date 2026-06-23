package container

import (
	"bytes"
	"fmt"
	"os"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"
	"strings"
	"time"
)

// newContainerNetworkController constructs a containerNetworkController with
// the default CommandFactory implementation. The controller is responsible
// for preparing container networking (veth creation and namespace setup)
// during container initialization.
func newContainerNetworkController() *containerNetworkController {
	return &containerNetworkController{
		commandFactory:   &utils.ExecCommandFactory{},
		waitProcessNetns: waitProcessNetnsReady,
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
	commandFactory   utils.CommandFactory
	waitProcessNetns func(containerId string, pid int, timeout time.Duration, pollInterval time.Duration) error
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

	rootless := isRootlessAnnotation(annotation)

	if networkConfig.Interface.Name == "" || networkConfig.Interface.IPv4.Address == "" {
		return nil
	}

	if rootless {
		waitProcessNetns := c.waitProcessNetns
		if waitProcessNetns == nil {
			waitProcessNetns = waitProcessNetnsReady
		}
		if err := waitProcessNetns(containerId, pid, 3*time.Second, 20*time.Millisecond); err != nil {
			return err
		}
	}

	// 2. create veth pair
	if err := c.createVethPair(containerId, pid, networkConfig, rootless); err != nil {
		return err
	}

	// 3. setup inside container
	if err := c.setupContainerNetns(containerId, pid, networkConfig, rootless); err != nil {
		return err
	}
	return nil
}

func runNetworkCommand(step string, cmd utils.CommandExecutor) error {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetStdout(&stdout)
	cmd.SetStderr(&stderr)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("network command %s failed: %w: stdout=%q stderr=%q", step, err, stdout.String(), stderr.String())
	}
	return nil
}

// createVethPair creates the veth pair used for container networking.
//
// Host-side operations performed:
//  1. Create a veth pair in the host network namespace
//  2. Move the peer interface into the target container network namespace
//  3. Attach the host-side veth to the specified bridge
//  4. Bring the host-side veth interface up
func (c *containerNetworkController) createVethPair(containerId string, pid int, networkConfig spec.NetConfigObject, rootless bool) error {
	pidStr := fmt.Sprint(pid)
	hostVethName := networkConfig.Interface.Name
	containerVethName := buildContainerVethPeerName(containerId)

	// 1. create veth pair in the host namespace first.
	//
	// Some iproute2/kernel combinations do not reliably accept
	// `peer ... netns <pid>` during veth creation, especially when the
	// target network namespace belongs to a process that also has a user
	// namespace. Splitting creation and movement gives us clearer failures
	// and uses the better-supported `ip link set <dev> netns <pid>` path.
	createVeth := c.commandFactory.Command("ip", "link", "add", "name", networkConfig.Interface.Name, "type", "veth", "peer", "name", containerVethName)
	//if err := createVeth.Run(); err != nil {
	if err := runNetworkCommand("create_veth_pair", createVeth); err != nil {
		return err
	}

	// 2. move peer to container netns
	netnsTarget := pidStr
	if rootless {
		netnsName := buildRootlessNetnsName(containerId)
		netnsPath := buildNamedNetnsPath(netnsName)

		cleanupNetns := c.commandFactory.Command("sh", "-c", fmt.Sprintf("umount %q 2>/dev/null || true; rm -f %q", netnsPath, netnsPath))
		if err := runNetworkCommand("cleanup_container_netns", cleanupNetns); err != nil {
			return err
		}

		ensureNetnsDir := c.commandFactory.Command("mkdir", "-p", "/run/netns")
		if err := runNetworkCommand("ensure_netns_dir", ensureNetnsDir); err != nil {
			return err
		}

		ensureNetnsFile := c.commandFactory.Command("touch", netnsPath)
		if err := runNetworkCommand("ensure_netns_file", ensureNetnsFile); err != nil {
			return err
		}

		bindNetns := c.commandFactory.Command("mount", "--bind", buildProcessNetnsPath(pid), netnsPath)
		if err := runNetworkCommand("bind_container_netns", bindNetns); err != nil {
			return err
		}

		netnsTarget = netnsName
	}

	movePeerToNetns := c.commandFactory.Command("ip", "link", "set", containerVethName, "netns", netnsTarget)
	if err := runNetworkCommand("move_veth_peer_to_netns", movePeerToNetns); err != nil {
		return err
	}

	if rootless {
		netnsPath := buildNamedNetnsPath(buildRootlessNetnsName(containerId))
		cleanupNetns := c.commandFactory.Command("sh", "-c", fmt.Sprintf("umount %q 2>/dev/null || true; rm -f %q", netnsPath, netnsPath))
		if err := runNetworkCommand("cleanup_container_netns", cleanupNetns); err != nil {
			return err
		}
	}

	// 3. attach veth to bridge
	attacheVeth := c.commandFactory.Command("ip", "link", "set", hostVethName, "master", networkConfig.BridgeInterface)
	//if err := attacheVeth.Run(); err != nil {
	if err := runNetworkCommand("attach_veth_to_bridge", attacheVeth); err != nil {
		return err
	}

	// 4. up veth
	upVeth := c.commandFactory.Command("ip", "link", "set", hostVethName, "up")
	//if err := upVeth.Run(); err != nil {
	if err := runNetworkCommand("up_host_veth", upVeth); err != nil {
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
func (c *containerNetworkController) setupContainerNetns(containerId string, pid int, networkConfig spec.NetConfigObject, rootless bool) error {
	pidStr := fmt.Sprint(pid)
	containerVethName := buildContainerVethPeerName(containerId)

	nsenterArgs := func(args ...string) []string {
		base := []string{"-t", pidStr}
		if rootless {
			base = append(base, "-U", "--setuid", "0", "--setgid", "0")
		}
		base = append(base, "-n")
		return append(base, args...)
	}

	// 1. up loopback i/f
	upLoopbackIf := c.commandFactory.Command("nsenter", nsenterArgs("ip", "link", "set", "lo", "up")...)
	//if err := upLoopbackIf.Run(); err != nil {
	if err := runNetworkCommand("up_loopback_in_netns", upLoopbackIf); err != nil {
		return err
	}

	// 2. rename veth
	renameVeth := c.commandFactory.Command("nsenter", nsenterArgs("ip", "link", "set", containerVethName, "name", "eth0")...)
	//if err := renameVeth.Run(); err != nil {
	if err := runNetworkCommand("rename_veth_in_netns", renameVeth); err != nil {
		return err
	}

	// 3. assign address
	assignAddr := c.commandFactory.Command("nsenter", nsenterArgs("ip", "addr", "add", networkConfig.Interface.IPv4.Address, "dev", "eth0")...)
	//if err := assignAddr.Run(); err != nil {
	if err := runNetworkCommand("assign_addr_in_netns", assignAddr); err != nil {
		return err
	}

	// 4. up veth
	upVeth := c.commandFactory.Command("nsenter", nsenterArgs("ip", "link", "set", "eth0", "up")...)
	//if err := upVeth.Run(); err != nil {
	if err := runNetworkCommand("up_veth_in_netns", upVeth); err != nil {
		return err
	}

	// 5. set gateway
	setGateway := c.commandFactory.Command("nsenter", nsenterArgs("ip", "route", "add", "default", "via", networkConfig.Interface.IPv4.Gateway)...)
	//if err := setGateway.Run(); err != nil {
	if err := runNetworkCommand("set_gateway_in_netns", setGateway); err != nil {
		return err
	}

	return nil
}

func waitProcessNetnsReady(containerId string, pid int, timeout time.Duration, pollInterval time.Duration) error {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	if pollInterval <= 0 {
		pollInterval = 20 * time.Millisecond
	}

	deadline := time.Now().Add(timeout)
	netnsPath := buildProcessNetnsPath(pid)
	procPath := fmt.Sprintf("/proc/%d", pid)
	var lastErr error

	for {
		if _, err := os.Stat(netnsPath); err == nil {
			return nil
		} else {
			lastErr = err
		}

		if time.Now().After(deadline) {
			return fmt.Errorf(
				"container init network namespace is not available: pid=%d proc=%q netns=%q last_error=%v init_log_tail=%q",
				pid,
				procPath,
				netnsPath,
				lastErr,
				readInitLogTail(containerId, 4096),
			)
		}

		time.Sleep(pollInterval)
	}
}

func readInitLogTail(containerId string, maxBytes int) string {
	if maxBytes <= 0 {
		maxBytes = 4096
	}
	data, err := os.ReadFile(utils.ContainerLogPath(containerId))
	if err != nil {
		return fmt.Sprintf("read init log failed: %v", err)
	}
	if len(data) > maxBytes {
		data = data[len(data)-maxBytes:]
	}
	return strings.TrimSpace(string(data))
}

func buildContainerVethPeerName(containerId string) string {
	if len(containerId) > 12 {
		containerId = containerId[:12]
	}
	return "rp_" + containerId
}

func buildProcessNetnsPath(pid int) string {
	return fmt.Sprintf("/proc/%d/ns/net", pid)
}

func buildRootlessNetnsName(containerId string) string {
	return "rn_" + buildContainerVethPeerName(containerId)
}

func buildNamedNetnsPath(netnsName string) string {
	return "/run/netns/" + netnsName
}

func isRootlessAnnotation(annotation spec.AnnotationObject) bool {
	if annotation.Rootless == "" {
		return false
	}
	var rootless spec.RootlessConfigObject
	if err := utils.StringToJson(annotation.Rootless, &rootless); err != nil {
		return false
	}
	return rootless.Enabled
}
