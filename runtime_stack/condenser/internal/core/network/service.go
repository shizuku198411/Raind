package network

import (
	"condenser/internal/core/policy"
	"condenser/internal/store/ipam"
	"condenser/internal/utils"
	"fmt"
	"slices"
	"strconv"
)

func NewNetworkService() *NetworkService {
	return &NetworkService{
		commandFactory: utils.NewCommandFactory(),
		ipamHandler:    ipam.NewIpamManager(ipam.NewIpamStore(utils.IpamStorePath)),
		policyHandler:  policy.NewwServicePolicy(),
	}
}

type NetworkService struct {
	commandFactory utils.CommandFactory
	ipamHandler    ipam.IpamHandler
	policyHandler  policy.PolicyServiceHandler
}

type RollbackFlag struct {
	StoreIpam             bool
	CreateBridgeInterface bool
}

func (s *NetworkService) CreateNewNetwork(param ServiceNewNetworkModel) (err error) {
	var rollback RollbackFlag
	defer func() error {
		if err != nil {
			if rollback.StoreIpam {
				rberr := s.ipamHandler.RemoveBridge(param.Bridge)
				if rberr != nil {
					return fmt.Errorf("rollback failed: " + rberr.Error())
				}
			}
			if rollback.CreateBridgeInterface {
				rberr := s.RemoveBridgeInterface(param.Bridge)
				if rberr != nil {
					return fmt.Errorf("rollback failed: " + rberr.Error())
				}
			}
			rberr := s.policyHandler.CommitPolicy()
			if rberr != nil {
				return fmt.Errorf("rollback failed: " + rberr.Error())
			}
			return err
		}
		return nil
	}()

	// 1. store ipam
	_, addr, err := s.ipamHandler.StoreBridge(param.Bridge)
	if err != nil {
		return err
	}
	rollback.StoreIpam = true

	// 2. create bridge interface
	err = s.CreateBridgeInterface(param.Bridge, addr)
	if err != nil {
		return err
	}
	rollback.CreateBridgeInterface = true

	// 3. refresh policy
	err = s.policyHandler.CommitPolicy()
	if err != nil {
		return err
	}
	return nil
}

func (s *NetworkService) RemoveNetwork(param ServiceRemoveNetworkModel) error {
	// check containers in network
	networkList, err := s.ipamHandler.GetNetworkList()
	if err != nil {
		return err
	}
	for _, n := range networkList {
		if n.Interface != param.Bridge {
			continue
		}
		if n.NumContainers != 0 {
			return fmt.Errorf("network: %s contains existing container", param.Bridge)
		}
	}
	// 1. remove bridge
	if err := s.RemoveBridgeInterface(param.Bridge); err != nil {
		return err
	}

	// 2. remove store
	if err := s.ipamHandler.RemoveBridge(param.Bridge); err != nil {
		return err
	}

	// 3. refresh policy
	if err := s.policyHandler.CommitPolicy(); err != nil {
		return err
	}

	return nil
}

func (s *NetworkService) CreateBridgeInterface(ifname string, addr string) error {
	// check if bridge already created
	check := s.commandFactory.Command("ip", "link", "show", ifname)
	if err := check.Run(); err == nil {
		// bridge already created, return
		return nil
	}

	// create bridge
	addLink := s.commandFactory.Command("ip", "link", "add", ifname, "type", "bridge")
	if err := addLink.Run(); err != nil {
		return err
	}
	// assign address
	assignAddr := s.commandFactory.Command("ip", "addr", "add", addr, "dev", ifname)
	if err := assignAddr.Run(); err != nil {
		return err
	}
	// up link
	upLink := s.commandFactory.Command("ip", "link", "set", ifname, "up")
	if err := upLink.Run(); err != nil {
		return err
	}

	return nil
}

func (s *NetworkService) RemoveBridgeInterface(ifname string) error {
	// check if bridge exist
	check := s.commandFactory.Command("ip", "link", "show", ifname)
	if err := check.Run(); err != nil {
		// bridge not exist
		return fmt.Errorf("bridge: %s not found", ifname)
	}

	// remove bridge
	remove := s.commandFactory.Command("ip", "link", "del", ifname)
	if err := remove.Run(); err != nil {
		return err
	}
	return nil
}

func (s *NetworkService) CreateMasqueradeRule(src string, dst string) error {
	// check if rule already exist
	check := s.commandFactory.Command("iptables", "-t", "nat", "-C", "POSTROUTING", "-s", src, "-o", dst, "-j", "MASQUERADE")
	if err := check.Run(); err == nil {
		// rule already exist
		return nil
	}

	// add rule
	add := s.commandFactory.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", src, "-o", dst, "-j", "MASQUERADE")
	if err := add.Run(); err != nil {
		return err
	}
	return nil
}

func (s *NetworkService) CreateRedirectDnsTrafficRule(forwarderIf string, forwarderAddr string) error {
	// check if rule already exist
	check1 := s.commandFactory.Command("iptables", "-t", "nat", "-C", "PREROUTING", "-i", forwarderIf, "-p", "udp", "--dport", "53", "-j", "DNAT", "--to-destination", forwarderAddr+":1053")
	if err := check1.Run(); err != nil {
		// rule not exist, create rule
		add1 := s.commandFactory.Command("iptables", "-t", "nat", "-A", "PREROUTING", "-i", forwarderIf, "-p", "udp", "--dport", "53", "-j", "DNAT", "--to-destination", forwarderAddr+":1053")
		if err := add1.Run(); err != nil {
			return err
		}
	}

	check2 := s.commandFactory.Command("iptables", "-t", "nat", "-C", "PREROUTING", "-i", forwarderIf, "-p", "tcp", "--dport", "53", "-j", "DNAT", "--to-destination", forwarderAddr+":1053")
	if err := check2.Run(); err != nil {
		// rule not exist, create rule
		add2 := s.commandFactory.Command("iptables", "-t", "nat", "-A", "PREROUTING", "-i", forwarderIf, "-p", "tcp", "--dport", "53", "-j", "DNAT", "--to-destination", forwarderAddr+":1053")
		if err := add2.Run(); err != nil {
			return err
		}
	}
	return nil
}

func (s *NetworkService) InsertInputRule(num int, ruleModel InputRuleModel, action string) error {
	ruleParam := []string{"-s", ruleModel.SourceAddr, "-d", ruleModel.DestAddr, "-j", action}
	if ruleModel.Protocol != "" {
		ruleParam = slices.Concat(ruleParam, []string{"-p", ruleModel.Protocol})
	}
	if ruleModel.SourcePort > 0 {
		ruleParam = slices.Concat(ruleParam, []string{"--sport", strconv.Itoa(ruleModel.SourcePort)})
	}
	if ruleModel.DestPort > 0 {
		ruleParam = slices.Concat(ruleParam, []string{"--dport", strconv.Itoa(ruleModel.DestPort)})
	}

	// check if rule already exist
	checkCmd := slices.Concat([]string{"iptables", "-C", "INPUT"}, ruleParam)
	check := s.commandFactory.Command(checkCmd[0], checkCmd[1:]...)
	if err := check.Run(); err == nil {
		// rule already exist
		return nil
	}

	// add rule
	addRuleCmd := slices.Concat([]string{"iptables", "-I", "INPUT", strconv.Itoa(num)}, ruleParam)
	addRule := s.commandFactory.Command(addRuleCmd[0], addRuleCmd[1:]...)
	if err := addRule.Run(); err != nil {
		return err
	}
	return nil
}

func (s *NetworkService) CreateForwardingRule(containerId string, parameter ServiceNetworkModel) error {
	// get container address
	host, bridge, addr, err := s.getContainerAddress(containerId)
	if err != nil {
		return err
	}

	// set rules
	if err := s.setForwardRules(host, bridge, addr, parameter); err != nil {
		return err
	}

	return nil
}

func (s *NetworkService) RemoveForwardingRule(containerId string, parameter ServiceNetworkModel) error {
	// get container address
	host, bridge, addr, err := s.getContainerAddress(containerId)
	if err != nil {
		return err
	}

	// remove rules
	if err := s.deleteForwardRules(host, bridge, addr, parameter); err != nil {
		return err
	}

	return nil
}

func (s *NetworkService) getContainerAddress(containerId string) (string, string, string, error) {
	host, bridge, addr, err := s.ipamHandler.GetContainerAddress(containerId)
	if err != nil {
		return "", "", "", err
	}
	return host, bridge, addr, nil
}

func (s *NetworkService) setForwardRules(hostInterface string, bridgeInterface string, containerAddr string, portParam ServiceNetworkModel) error {
	// 1. dnat
	dnatRuleCmd := []string{
		"iptables",
		"-t", "nat",
		"-A", "PREROUTING",
		"-i", hostInterface,
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-j", "DNAT",
		"--to-destination", containerAddr + ":" + portParam.ContainerPort,
	}
	dnatRule := s.commandFactory.Command(dnatRuleCmd[0], dnatRuleCmd[1:]...)
	if err := dnatRule.Run(); err != nil {
		return err
	}

	// 2. dnat for local host traffic from host namespace
	dnatOutputRuleCmd := []string{
		"iptables",
		"-t", "nat",
		"-A", "OUTPUT",
		"-m", "addrtype",
		"--dst-type", "LOCAL",
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-j", "DNAT",
		"--to-destination", containerAddr + ":" + portParam.ContainerPort,
	}
	dnatOutputRule := s.commandFactory.Command(dnatOutputRuleCmd[0], dnatOutputRuleCmd[1:]...)
	if err := dnatOutputRule.Run(); err != nil {
		return err
	}

	// 3. dnat for local host traffic from containers
	dnatBridgeRuleCmd := []string{
		"iptables",
		"-t", "nat",
		"-A", "PREROUTING",
		"-i", bridgeInterface,
		"-m", "addrtype",
		"--dst-type", "LOCAL",
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-j", "DNAT",
		"--to-destination", containerAddr + ":" + portParam.ContainerPort,
	}
	dnatBridgeRule := s.commandFactory.Command(dnatBridgeRuleCmd[0], dnatBridgeRuleCmd[1:]...)
	if err := dnatBridgeRule.Run(); err != nil {
		return err
	}

	// 4. allow forward: in
	forwardInCmd := []string{
		"iptables",
		"-A", "FORWARD",
		"-i", hostInterface,
		"-o", bridgeInterface,
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-d", containerAddr,
		"-j", "ACCEPT",
	}
	forwardIn := s.commandFactory.Command(forwardInCmd[0], forwardInCmd[1:]...)
	if err := forwardIn.Run(); err != nil {
		return err
	}

	// 5. allow forwarded dnat traffic within same bridge (container -> hostAddr -> container)
	forwardHairpinCmd := []string{
		"iptables",
		"-I", "FORWARD", "1",
		"-i", bridgeInterface,
		"-o", bridgeInterface,
		"-p", portParam.Protocol,
		"-m", "conntrack",
		"--ctstate", "DNAT",
		"--dport", portParam.ContainerPort,
		"-d", containerAddr,
		"-j", "ACCEPT",
	}
	forwardHairpin := s.commandFactory.Command(forwardHairpinCmd[0], forwardHairpinCmd[1:]...)
	if err := forwardHairpin.Run(); err != nil {
		return err
	}

	// 6. allow forward: out
	forwardOutCmd := []string{
		"iptables",
		"-A", "FORWARD",
		"-o", hostInterface,
		"-i", bridgeInterface,
		"-p", portParam.Protocol,
		"--sport", portParam.HostPort,
		"-s", containerAddr,
		"-j", "ACCEPT",
	}
	forwardOut := s.commandFactory.Command(forwardOutCmd[0], forwardOutCmd[1:]...)
	if err := forwardOut.Run(); err != nil {
		return err
	}

	return nil
}

func (s *NetworkService) deleteForwardRules(hostInterface string, bridgeInterface string, containerAddr string, portParam ServiceNetworkModel) error {
	// 1. dnat
	dnatRuleCmd := []string{
		"iptables",
		"-t", "nat",
		"-D", "PREROUTING",
		"-i", hostInterface,
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-j", "DNAT",
		"--to-destination", containerAddr + ":" + portParam.ContainerPort,
	}
	dnatRule := s.commandFactory.Command(dnatRuleCmd[0], dnatRuleCmd[1:]...)
	if err := dnatRule.Run(); err != nil {
		return err
	}

	// 2. dnat for local host traffic from host namespace
	dnatOutputRuleCmd := []string{
		"iptables",
		"-t", "nat",
		"-D", "OUTPUT",
		"-m", "addrtype",
		"--dst-type", "LOCAL",
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-j", "DNAT",
		"--to-destination", containerAddr + ":" + portParam.ContainerPort,
	}
	dnatOutputRule := s.commandFactory.Command(dnatOutputRuleCmd[0], dnatOutputRuleCmd[1:]...)
	if err := dnatOutputRule.Run(); err != nil {
		return err
	}

	// 3. dnat for local host traffic from containers
	dnatBridgeRuleCmd := []string{
		"iptables",
		"-t", "nat",
		"-D", "PREROUTING",
		"-i", bridgeInterface,
		"-m", "addrtype",
		"--dst-type", "LOCAL",
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-j", "DNAT",
		"--to-destination", containerAddr + ":" + portParam.ContainerPort,
	}
	dnatBridgeRule := s.commandFactory.Command(dnatBridgeRuleCmd[0], dnatBridgeRuleCmd[1:]...)
	if err := dnatBridgeRule.Run(); err != nil {
		return err
	}

	// 4. allow forward: in
	forwardInCmd := []string{
		"iptables",
		"-D", "FORWARD",
		"-i", hostInterface,
		"-o", bridgeInterface,
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-d", containerAddr,
		"-j", "ACCEPT",
	}
	forwardIn := s.commandFactory.Command(forwardInCmd[0], forwardInCmd[1:]...)
	if err := forwardIn.Run(); err != nil {
		return err
	}

	// 5. allow forwarded dnat traffic within same bridge (container -> hostAddr -> container)
	forwardHairpinCmd := []string{
		"iptables",
		"-D", "FORWARD",
		"-i", bridgeInterface,
		"-o", bridgeInterface,
		"-p", portParam.Protocol,
		"-m", "conntrack",
		"--ctstate", "DNAT",
		"--dport", portParam.ContainerPort,
		"-d", containerAddr,
		"-j", "ACCEPT",
	}
	forwardHairpin := s.commandFactory.Command(forwardHairpinCmd[0], forwardHairpinCmd[1:]...)
	if err := forwardHairpin.Run(); err != nil {
		return err
	}

	// 6. allow forward: out
	forwardOutCmd := []string{
		"iptables",
		"-D", "FORWARD",
		"-o", hostInterface,
		"-i", bridgeInterface,
		"-p", portParam.Protocol,
		"--sport", portParam.HostPort,
		"-s", containerAddr,
		"-j", "ACCEPT",
	}
	forwardOut := s.commandFactory.Command(forwardOutCmd[0], forwardOutCmd[1:]...)
	if err := forwardOut.Run(); err != nil {
		return err
	}

	return nil
}
