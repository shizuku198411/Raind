package network

import (
	"fmt"
	"raind/internal/condenser/core/policy"
	"raind/internal/condenser/store/ipam"
	"raind/internal/condenser/utils"
	"slices"
	"strconv"
	"strings"
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
	if err := s.ensureNoUnmanagedNamespaceBridge(); err != nil {
		return err
	}

	var rollback RollbackFlag
	defer func() error {
		if err != nil {
			if rollback.StoreIpam {
				rberr := s.ipamHandler.RemoveBridge(param.Bridge)
				if rberr != nil {
					return fmt.Errorf("rollback failed: %w", rberr)
				}
			}
			if rollback.CreateBridgeInterface {
				rberr := s.RemoveBridgeInterface(param.Bridge)
				if rberr != nil {
					return fmt.Errorf("rollback failed: %w", rberr)
				}
			}
			rberr := s.policyHandler.CommitPolicy()
			if rberr != nil {
				return fmt.Errorf("rollback failed: %w", rberr)
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

	// 3. Setup DNS redirect for the newly-created network
	_, dnsProxyAddr, _, err := s.ipamHandler.GetDnsProxyInfo()
	if err != nil {
		return err
	}
	if err := s.CreateRedirectDnsTrafficRule(param.Bridge, dnsProxyAddr); err != nil {
		return err
	}

	// 4. refresh policy
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

	// 1. remove DNS redirect rules
	_, dnsProxyAddr, _, err := s.ipamHandler.GetDnsProxyInfo()
	if err != nil {
		return err
	}
	if err := s.RemoveRedirectDnsTrafficRule(param.Bridge, dnsProxyAddr); err != nil {
		return err
	}

	// 2. remove bridge
	if err := s.RemoveBridgeInterface(param.Bridge); err != nil {
		return err
	}

	// 3. remove store
	if err := s.ipamHandler.RemoveBridge(param.Bridge); err != nil {
		return err
	}

	// 4. refresh policy
	if err := s.policyHandler.CommitPolicy(); err != nil {
		return err
	}

	return nil
}

func (s *NetworkService) RemoveRedirectDnsTrafficRule(forwarderIf string, forwarderAddr string) error {
	if err := s.removeRedirectDnsTrafficRule(forwarderIf, forwarderAddr, "udp"); err != nil {
		return err
	}
	if err := s.removeRedirectDnsTrafficRule(forwarderIf, forwarderAddr, "tcp"); err != nil {
		return err
	}
	return nil
}

func (s *NetworkService) removeRedirectDnsTrafficRule(forwarderIf string, forwarderAddr string, proto string) error {
	args := []string{"PREROUTING", "-i", forwarderIf, "-p", proto, "--dport", "53", "-j", "DNAT", "--to-destination", forwarderAddr + ":1053"}

	// Treat a missing rule as success. This keeps network deletion idempotent and
	// allows cleanup to proceed for networks created before DNS redirect cleanup
	// was implemented.
	check := s.commandFactory.Command("iptables", append([]string{"-t", "nat", "-C"}, args...)...)
	if err := check.Run(); err != nil {
		return nil
	}

	remove := s.commandFactory.Command("iptables", append([]string{"-t", "nat", "-D"}, args...)...)
	if err := remove.Run(); err != nil {
		return err
	}
	return nil
}

func (s *NetworkService) ensureNoUnmanagedNamespaceBridge() error {
	networkList, err := s.ipamHandler.GetNetworkList()
	if err != nil {
		return err
	}
	managed := make(map[string]struct{}, len(networkList))
	for _, n := range networkList {
		managed[n.Interface] = struct{}{}
	}

	out, err := s.commandFactory.Command("ip", "-o", "link", "show").Output()
	if err != nil {
		return err
	}
	for _, ifname := range parseLinkNames(string(out)) {
		if !strings.HasPrefix(ifname, "rns") {
			continue
		}
		if _, ok := managed[ifname]; ok {
			continue
		}
		return fmt.Errorf("unmanaged namespace bridge exists: %s; remove stale bridge before creating a network", ifname)
	}
	return nil
}

func parseLinkNames(output string) []string {
	lines := strings.Split(output, "\n")
	names := make([]string, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, ": ", 3)
		if len(parts) < 3 {
			continue
		}
		name := strings.TrimSpace(parts[1])
		if i := strings.Index(name, "@"); i >= 0 {
			name = name[:i]
		}
		if name != "" {
			names = append(names, name)
		}
	}
	return names
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
	dnatRuleParam := []string{
		"-i", hostInterface,
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-j", "DNAT",
		"--to-destination", containerAddr + ":" + portParam.ContainerPort,
	}
	if err := s.addIptablesRuleIfMissing("nat", "PREROUTING", "append", dnatRuleParam); err != nil {
		return err
	}

	// 2. dnat for local host traffic from host namespace
	dnatOutputRuleParam := []string{
		"-m", "addrtype",
		"--dst-type", "LOCAL",
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-j", "DNAT",
		"--to-destination", containerAddr + ":" + portParam.ContainerPort,
	}
	if err := s.addIptablesRuleIfMissing("nat", "OUTPUT", "append", dnatOutputRuleParam); err != nil {
		return err
	}

	// 3. dnat for local host traffic from containers
	dnatBridgeRuleParam := []string{
		"-i", bridgeInterface,
		"-m", "addrtype",
		"--dst-type", "LOCAL",
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-j", "DNAT",
		"--to-destination", containerAddr + ":" + portParam.ContainerPort,
	}
	if err := s.addIptablesRuleIfMissing("nat", "PREROUTING", "append", dnatBridgeRuleParam); err != nil {
		return err
	}

	// 4. masquerade localhost-originated traffic after OUTPUT DNAT.
	// Without this, localhost/127.0.0.1 published-port access can leave the
	// host namespace with a loopback source address and stall before the
	// container can return traffic. External host-interface traffic keeps the
	// existing forwarding path unchanged.
	localhostMasqueradeParam := []string{
		"-s", "127.0.0.0/8",
		"-d", containerAddr,
		"-p", portParam.Protocol,
		"--dport", portParam.ContainerPort,
		"-j", "MASQUERADE",
	}
	if err := s.addIptablesRuleIfMissing("nat", "POSTROUTING", "append", localhostMasqueradeParam); err != nil {
		return err
	}

	// 5. allow forward: in
	forwardInParam := []string{
		"-i", hostInterface,
		"-o", bridgeInterface,
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-d", containerAddr,
		"-j", "ACCEPT",
	}
	if err := s.addIptablesRuleIfMissing("", "FORWARD", "append", forwardInParam); err != nil {
		return err
	}

	// 6. allow forwarded dnat traffic within same bridge (container -> hostAddr -> container)
	forwardHairpinParam := []string{
		"-i", bridgeInterface,
		"-o", bridgeInterface,
		"-p", portParam.Protocol,
		"-m", "conntrack",
		"--ctstate", "DNAT",
		"--dport", portParam.ContainerPort,
		"-d", containerAddr,
		"-j", "ACCEPT",
	}
	if err := s.addIptablesRuleIfMissing("", "FORWARD", "insert-first", forwardHairpinParam); err != nil {
		return err
	}

	// 7. allow forward: out
	forwardOutParam := []string{
		"-o", hostInterface,
		"-i", bridgeInterface,
		"-p", portParam.Protocol,
		"--sport", portParam.HostPort,
		"-s", containerAddr,
		"-j", "ACCEPT",
	}
	if err := s.addIptablesRuleIfMissing("", "FORWARD", "append", forwardOutParam); err != nil {
		return err
	}

	return nil
}

func (s *NetworkService) deleteForwardRules(hostInterface string, bridgeInterface string, containerAddr string, portParam ServiceNetworkModel) error {
	// 1. dnat
	dnatRuleParam := []string{
		"-i", hostInterface,
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-j", "DNAT",
		"--to-destination", containerAddr + ":" + portParam.ContainerPort,
	}
	if err := s.deleteIptablesRuleIfExists("nat", "PREROUTING", dnatRuleParam); err != nil {
		return err
	}

	// 2. dnat for local host traffic from host namespace
	dnatOutputRuleParam := []string{
		"-m", "addrtype",
		"--dst-type", "LOCAL",
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-j", "DNAT",
		"--to-destination", containerAddr + ":" + portParam.ContainerPort,
	}
	if err := s.deleteIptablesRuleIfExists("nat", "OUTPUT", dnatOutputRuleParam); err != nil {
		return err
	}

	// 3. dnat for local host traffic from containers
	dnatBridgeRuleParam := []string{
		"-i", bridgeInterface,
		"-m", "addrtype",
		"--dst-type", "LOCAL",
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-j", "DNAT",
		"--to-destination", containerAddr + ":" + portParam.ContainerPort,
	}
	if err := s.deleteIptablesRuleIfExists("nat", "PREROUTING", dnatBridgeRuleParam); err != nil {
		return err
	}

	// 4. masquerade localhost-originated traffic after OUTPUT DNAT.
	localhostMasqueradeParam := []string{
		"-s", "127.0.0.0/8",
		"-d", containerAddr,
		"-p", portParam.Protocol,
		"--dport", portParam.ContainerPort,
		"-j", "MASQUERADE",
	}
	if err := s.deleteIptablesRuleIfExists("nat", "POSTROUTING", localhostMasqueradeParam); err != nil {
		return err
	}

	// 5. allow forward: in
	forwardInParam := []string{
		"-i", hostInterface,
		"-o", bridgeInterface,
		"-p", portParam.Protocol,
		"--dport", portParam.HostPort,
		"-d", containerAddr,
		"-j", "ACCEPT",
	}
	if err := s.deleteIptablesRuleIfExists("", "FORWARD", forwardInParam); err != nil {
		return err
	}

	// 6. allow forwarded dnat traffic within same bridge (container -> hostAddr -> container)
	forwardHairpinParam := []string{
		"-i", bridgeInterface,
		"-o", bridgeInterface,
		"-p", portParam.Protocol,
		"-m", "conntrack",
		"--ctstate", "DNAT",
		"--dport", portParam.ContainerPort,
		"-d", containerAddr,
		"-j", "ACCEPT",
	}
	if err := s.deleteIptablesRuleIfExists("", "FORWARD", forwardHairpinParam); err != nil {
		return err
	}

	// 7. allow forward: out
	forwardOutParam := []string{
		"-o", hostInterface,
		"-i", bridgeInterface,
		"-p", portParam.Protocol,
		"--sport", portParam.HostPort,
		"-s", containerAddr,
		"-j", "ACCEPT",
	}
	if err := s.deleteIptablesRuleIfExists("", "FORWARD", forwardOutParam); err != nil {
		return err
	}

	return nil
}

func (s *NetworkService) addIptablesRuleIfMissing(table string, chain string, mode string, ruleParam []string) error {
	checkCmd := buildIptablesRuleCommand(table, "-C", chain, "", ruleParam)
	check := s.commandFactory.Command(checkCmd[0], checkCmd[1:]...)
	if err := check.Run(); err == nil {
		return nil
	}

	var addCmd []string
	switch mode {
	case "insert-first":
		addCmd = buildIptablesRuleCommand(table, "-I", chain, "1", ruleParam)
	default:
		addCmd = buildIptablesRuleCommand(table, "-A", chain, "", ruleParam)
	}
	add := s.commandFactory.Command(addCmd[0], addCmd[1:]...)
	if err := add.Run(); err != nil {
		return err
	}
	return nil
}

func (s *NetworkService) deleteIptablesRuleIfExists(table string, chain string, ruleParam []string) error {
	checkCmd := buildIptablesRuleCommand(table, "-C", chain, "", ruleParam)
	check := s.commandFactory.Command(checkCmd[0], checkCmd[1:]...)
	if err := check.Run(); err != nil {
		return nil
	}

	deleteCmd := buildIptablesRuleCommand(table, "-D", chain, "", ruleParam)
	deleteRule := s.commandFactory.Command(deleteCmd[0], deleteCmd[1:]...)
	if err := deleteRule.Run(); err != nil {
		return err
	}
	return nil
}

func buildIptablesRuleCommand(table string, op string, chain string, position string, ruleParam []string) []string {
	cmd := []string{"iptables"}
	if table != "" {
		cmd = append(cmd, "-t", table)
	}
	cmd = append(cmd, op, chain)
	if position != "" {
		cmd = append(cmd, position)
	}
	cmd = append(cmd, ruleParam...)
	return cmd
}
