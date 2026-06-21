package service

import (
	"fmt"
	"log"
	"raind/internal/condenser/core/container"
	"raind/internal/condenser/store/ipam"
	"raind/internal/condenser/store/psm"
	"raind/internal/condenser/store/ssm"
	"raind/internal/condenser/utils"
	"sort"
	"strconv"
	"strings"
	"time"
)

func NewServiceController() *ServiceController {
	return &ServiceController{
		psmHandler:       psm.NewPsmManager(psm.NewPsmStore(utils.PsmStorePath)),
		ssmHandler:       ssm.NewSsmManager(ssm.NewSsmStore(utils.SsmStorePath)),
		containerHandler: container.NewContaierService(),
		ipamHandler:      ipam.NewIpamManager(ipam.NewIpamStore(utils.IpamStorePath)),
		commandFactory:   utils.NewCommandFactory(),
		interval:         5 * time.Second,
		lastState:        map[string]string{},
		lastServices:     map[string]ssm.ServiceInfo{},
	}
}

type ServiceController struct {
	psmHandler       psm.PsmHandler
	ssmHandler       ssm.SsmHandler
	containerHandler container.ContainerServiceHandler
	ipamHandler      ipam.IpamHandler
	commandFactory   utils.CommandFactory
	interval         time.Duration
	lastState        map[string]string
	lastServices     map[string]ssm.ServiceInfo
}

func (c *ServiceController) Start() {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for range ticker.C {
		if err := c.reconcileOnce(); err != nil {
			log.Printf("service controller reconcile failed: %v", err)
		}
	}
}

func (c *ServiceController) reconcileOnce() error {
	services, err := c.ssmHandler.GetServiceList()
	if err != nil {
		return err
	}
	if len(services) > 0 {
		pods, err := c.psmHandler.GetPodList()
		if err != nil {
			return err
		}

		for _, svc := range services {
			endpoints, err := c.buildEndpoints(svc, pods)
			if err != nil {
				log.Printf("service controller endpoints failed: serviceId=%s err=%v", svc.ServiceId, err)
				continue
			}
			stateKey := c.buildStateKey(svc, endpoints)
			if prev := c.lastState[svc.ServiceId]; prev != stateKey {
				if _, ok := c.lastState[svc.ServiceId]; ok {
					c.cleanupService(svc.ServiceId)
				}
				c.lastState[svc.ServiceId] = stateKey
				c.lastServices[svc.ServiceId] = svc
				if err := c.applyRules(svc, endpoints); err != nil {
					log.Printf("service controller apply failed: serviceId=%s err=%v", svc.ServiceId, err)
				}
				log.Printf("service endpoints updated: serviceId=%s name=%s endpoints=%v", svc.ServiceId, svc.Name, endpoints)
			}
		}
	}

	// cleanup removed services
	for serviceId := range c.lastState {
		if !c.serviceExists(serviceId, services) {
			c.cleanupService(serviceId)
			delete(c.lastState, serviceId)
			delete(c.lastServices, serviceId)
		}
	}

	return nil
}

type svcEndpoint struct {
	Addr          string
	HostInterface string
	Bridge        string
}

func (c *ServiceController) buildEndpoints(svc ssm.ServiceInfo, pods []psm.PodInfo) ([]svcEndpoint, error) {
	var endpoints []svcEndpoint
	for _, p := range pods {
		if p.Namespace != svc.Namespace {
			continue
		}
		if !labelsMatch(svc.Selector, p.Labels) {
			continue
		}
		infraId, ok := c.getReadyInfraContainerId(p)
		if !ok {
			continue
		}
		host, bridge, addr, err := c.ipamHandler.GetContainerAddress(infraId)
		if err != nil {
			continue
		}
		endpoints = append(endpoints, svcEndpoint{
			Addr:          addr,
			HostInterface: host,
			Bridge:        bridge,
		})
	}
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].Addr < endpoints[j].Addr
	})
	return endpoints, nil
}

func (c *ServiceController) getReadyInfraContainerId(p psm.PodInfo) (string, bool) {
	if p.State != psm.PodStateRunning || p.StoppedByUser {
		return "", false
	}

	containers, err := c.containerHandler.GetContainersByPodId(p.PodId)
	if err != nil {
		return "", false
	}
	if len(containers) == 0 {
		return "", false
	}

	var (
		infraId       string
		runningByName = map[string]struct{}{}
		memberCount   int
	)
	for _, cinfo := range containers {
		if strings.HasPrefix(cinfo.Name, utils.PodInfraContainerNamePrefix) {
			if cinfo.State != psm.ContainerStateRunning {
				return "", false
			}
			infraId = cinfo.ContainerId
			continue
		}
		memberCount++
		if cinfo.State != psm.ContainerStateRunning {
			return "", false
		}
		runningByName[cinfo.Name] = struct{}{}
	}
	if infraId == "" || memberCount == 0 {
		return "", false
	}
	if !c.expectedMembersRunning(p, runningByName) {
		return "", false
	}
	return infraId, true
}

func (c *ServiceController) expectedMembersRunning(p psm.PodInfo, runningByName map[string]struct{}) bool {
	if p.TemplateId == "" {
		return true
	}
	tpl, err := c.psmHandler.GetPodTemplate(p.TemplateId)
	if err != nil {
		return false
	}

	for _, spec := range tpl.Spec.Containers {
		if spec.Name == "" {
			continue
		}
		expectedName := buildPodMemberName(spec.Name, p.PodId)
		if _, ok := runningByName[expectedName]; !ok {
			return false
		}
	}
	return true
}

func buildPodMemberName(baseName, podId string) string {
	if baseName == "" {
		return baseName
	}
	suffix := podId
	if len(suffix) > 8 {
		suffix = suffix[len(suffix)-8:]
	}
	return baseName + "-" + suffix
}

func (c *ServiceController) getInfraContainerId(podId string) (string, error) {
	containers, err := c.containerHandler.GetContainersByPodId(podId)
	if err != nil {
		return "", err
	}
	for _, cinfo := range containers {
		if strings.HasPrefix(cinfo.Name, utils.PodInfraContainerNamePrefix) {
			return cinfo.ContainerId, nil
		}
	}
	return "", containerNotFound(podId)
}

func labelsMatch(selector, labels map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

func (c *ServiceController) buildStateKey(svc ssm.ServiceInfo, endpoints []svcEndpoint) string {
	var parts []string
	parts = append(parts, "type="+serviceType(svc))
	if svc.ClusterIP != "" {
		parts = append(parts, "clusterIP="+svc.ClusterIP)
	}
	for _, p := range svc.Ports {
		proto := strings.ToLower(p.Protocol)
		parts = append(parts, fmt.Sprintf("%d:%d/%s", p.Port, p.TargetPort, proto))
	}
	for _, e := range endpoints {
		parts = append(parts, e.Addr)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}

func (c *ServiceController) applyRules(svc ssm.ServiceInfo, endpoints []svcEndpoint) error {
	forwardChain := c.serviceForwardChainName(svc.ServiceId)
	if err := c.ensureForwardChain(forwardChain); err != nil {
		return err
	}
	if err := c.flushForwardChain(forwardChain); err != nil {
		return err
	}
	_ = c.deleteForwardJumpRule(forwardChain)
	if err := c.addForwardJumpRule(forwardChain); err != nil {
		return err
	}

	snatChain := c.serviceSNATChainName(svc.ServiceId)
	if serviceType(svc) == ssm.ServiceTypeNodePort {
		if err := c.ensureSNATChain(snatChain); err != nil {
			return err
		}
		if err := c.flushSNATChain(snatChain); err != nil {
			return err
		}
		_ = c.deleteSNATJumpRule(snatChain)
		if err := c.addSNATJumpRule(snatChain); err != nil {
			return err
		}
	} else {
		_ = c.deleteSNATJumpRule(snatChain)
		_ = c.flushSNATChain(snatChain)
		_ = c.deleteSNATChain(snatChain)
	}

	for _, port := range svc.Ports {
		if port.Port == 0 || port.TargetPort == 0 {
			continue
		}
		proto := strings.ToLower(port.Protocol)
		if proto == "" {
			proto = "tcp"
		}
		chain := c.serviceChainName(svc.ServiceId, port.Port)
		if err := c.ensureChain(chain); err != nil {
			return err
		}
		if err := c.flushChain(chain); err != nil {
			return err
		}
		_ = c.deleteServiceJumpRule(svc, chain, proto, port.Port)
		if err := c.addServiceJumpRule(svc, chain, proto, port.Port); err != nil {
			return err
		}
		if len(endpoints) == 0 {
			continue
		}
		prob := 1.0 / float64(len(endpoints))
		for i, ep := range endpoints {
			if i < len(endpoints)-1 {
				if err := c.addEndpointRule(chain, ep.Addr, port.TargetPort, proto, prob); err != nil {
					return err
				}
			} else {
				if err := c.addEndpointRule(chain, ep.Addr, port.TargetPort, proto, 0); err != nil {
					return err
				}
			}
			_ = c.addForwardRules(forwardChain, ep, port.TargetPort, proto)
			if serviceType(svc) == ssm.ServiceTypeNodePort {
				_ = c.addLocalhostSNATRule(snatChain, ep.Addr, port.TargetPort, proto)
			}
		}
	}
	return nil
}

func serviceType(svc ssm.ServiceInfo) string {
	return ssm.NormalizeServiceType(svc.Type)
}

func (c *ServiceController) addServiceJumpRule(svc ssm.ServiceInfo, chain, proto string, port int) error {
	if serviceType(svc) == ssm.ServiceTypeClusterIP {
		return c.addClusterIPJumpRule(chain, svc.ClusterIP, proto, port)
	}
	return c.addNodePortJumpRule(chain, proto, port)
}

func (c *ServiceController) deleteServiceJumpRule(svc ssm.ServiceInfo, chain, proto string, port int) error {
	if serviceType(svc) == ssm.ServiceTypeClusterIP {
		return c.deleteClusterIPJumpRule(chain, svc.ClusterIP, proto, port)
	}
	return c.deleteNodePortJumpRule(chain, proto, port)
}

func (c *ServiceController) serviceChainName(serviceId string, port int) string {
	id := serviceId
	if len(id) > 8 {
		id = id[:8]
	}
	return "RAIND-SVC-" + id + "-" + itoa(port)
}

func (c *ServiceController) serviceForwardChainName(serviceId string) string {
	id := serviceId
	if len(id) > 8 {
		id = id[:8]
	}
	return "RAIND-FWD-" + id
}

func (c *ServiceController) serviceSNATChainName(serviceId string) string {
	id := serviceId
	if len(id) > 8 {
		id = id[:8]
	}
	return "RAIND-SNAT-" + id
}

func (c *ServiceController) serviceExists(serviceId string, list []ssm.ServiceInfo) bool {
	for _, s := range list {
		if s.ServiceId == serviceId {
			return true
		}
	}
	return false
}

func (c *ServiceController) cleanupService(serviceId string) {
	svc, ok := c.lastServices[serviceId]
	if !ok {
		return
	}
	c.cleanupServiceRules(svc)
}

func (c *ServiceController) cleanupServiceRules(svc ssm.ServiceInfo) {
	forwardChain := c.serviceForwardChainName(svc.ServiceId)
	_ = c.deleteForwardJumpRule(forwardChain)
	_ = c.flushForwardChain(forwardChain)
	_ = c.deleteForwardChain(forwardChain)

	snatChain := c.serviceSNATChainName(svc.ServiceId)
	_ = c.deleteSNATJumpRule(snatChain)
	_ = c.flushSNATChain(snatChain)
	_ = c.deleteSNATChain(snatChain)

	for _, p := range svc.Ports {
		if p.Port == 0 {
			continue
		}
		chain := c.serviceChainName(svc.ServiceId, p.Port)
		_ = c.deleteServiceJumpRule(svc, chain, "tcp", p.Port)
		_ = c.deleteServiceJumpRule(svc, chain, "udp", p.Port)
		_ = c.flushChain(chain)
		_ = c.deleteChain(chain)
	}
}

func (c *ServiceController) deleteChain(chain string) error {
	cmd := c.commandFactory.Command("iptables", "-t", "nat", "-X", chain)
	return cmd.Run()
}

func (c *ServiceController) ensureChain(chain string) error {
	cmd := c.commandFactory.Command("iptables", "-t", "nat", "-N", chain)
	_ = cmd.Run()
	return nil
}

func (c *ServiceController) flushChain(chain string) error {
	cmd := c.commandFactory.Command("iptables", "-t", "nat", "-F", chain)
	return cmd.Run()
}

func (c *ServiceController) ensureForwardChain(chain string) error {
	cmd := c.commandFactory.Command("iptables", "-N", chain)
	_ = cmd.Run()
	return nil
}

func (c *ServiceController) flushForwardChain(chain string) error {
	cmd := c.commandFactory.Command("iptables", "-F", chain)
	return cmd.Run()
}

func (c *ServiceController) deleteForwardChain(chain string) error {
	cmd := c.commandFactory.Command("iptables", "-X", chain)
	return cmd.Run()
}

func (c *ServiceController) ensureSNATChain(chain string) error {
	cmd := c.commandFactory.Command("iptables", "-t", "nat", "-N", chain)
	_ = cmd.Run()
	return nil
}

func (c *ServiceController) flushSNATChain(chain string) error {
	cmd := c.commandFactory.Command("iptables", "-t", "nat", "-F", chain)
	return cmd.Run()
}

func (c *ServiceController) deleteSNATChain(chain string) error {
	cmd := c.commandFactory.Command("iptables", "-t", "nat", "-X", chain)
	_ = cmd.Run()
	return nil
}

func (c *ServiceController) addSNATJumpRule(chain string) error {
	cmd := c.commandFactory.Command(
		"iptables", "-t", "nat", "-A", "POSTROUTING",
		"-s", "127.0.0.0/8",
		"-j", chain,
	)
	return cmd.Run()
}

func (c *ServiceController) deleteSNATJumpRule(chain string) error {
	cmd := c.commandFactory.Command(
		"iptables", "-t", "nat", "-D", "POSTROUTING",
		"-s", "127.0.0.0/8",
		"-j", chain,
	)
	_ = cmd.Run()
	return nil
}

func (c *ServiceController) addLocalhostSNATRule(chain, endpointAddr string, targetPort int, proto string) error {
	cmd := c.commandFactory.Command(
		"iptables", "-t", "nat", "-A", chain,
		"-s", "127.0.0.0/8",
		"-d", endpointAddr,
		"-p", proto,
		"--dport", itoa(targetPort),
		"-j", "MASQUERADE",
	)
	return cmd.Run()
}

func (c *ServiceController) addForwardJumpRule(chain string) error {
	cmd := c.commandFactory.Command("iptables", "-I", "FORWARD", "1", "-j", chain)
	return cmd.Run()
}

func (c *ServiceController) deleteForwardJumpRule(chain string) error {
	cmd := c.commandFactory.Command("iptables", "-D", "FORWARD", "-j", chain)
	_ = cmd.Run()
	return nil
}

func (c *ServiceController) deleteNodePortJumpRule(chain, proto string, port int) error {
	cmd := c.commandFactory.Command(
		"iptables", "-t", "nat", "-D", "PREROUTING",
		"-p", proto, "--dport", itoa(port),
		"-j", chain,
	)
	_ = cmd.Run()
	cmd = c.commandFactory.Command(
		"iptables", "-t", "nat", "-D", "OUTPUT",
		"-m", "addrtype", "--dst-type", "LOCAL",
		"-p", proto, "--dport", itoa(port),
		"-j", chain,
	)
	_ = cmd.Run()
	return nil
}

func (c *ServiceController) addNodePortJumpRule(chain, proto string, port int) error {
	cmd := c.commandFactory.Command(
		"iptables", "-t", "nat", "-A", "PREROUTING",
		"-p", proto, "--dport", itoa(port),
		"-j", chain,
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	cmd = c.commandFactory.Command(
		"iptables", "-t", "nat", "-A", "OUTPUT",
		"-m", "addrtype", "--dst-type", "LOCAL",
		"-p", proto, "--dport", itoa(port),
		"-j", chain,
	)
	return cmd.Run()
}

func (c *ServiceController) deleteClusterIPJumpRule(chain, clusterIP, proto string, port int) error {
	if clusterIP == "" {
		return nil
	}
	dst := clusterIP + "/32"
	cmd := c.commandFactory.Command(
		"iptables", "-t", "nat", "-D", "PREROUTING",
		"-d", dst,
		"-p", proto, "--dport", itoa(port),
		"-j", chain,
	)
	_ = cmd.Run()
	cmd = c.commandFactory.Command(
		"iptables", "-t", "nat", "-D", "OUTPUT",
		"-d", dst,
		"-p", proto, "--dport", itoa(port),
		"-j", chain,
	)
	_ = cmd.Run()
	return nil
}

func (c *ServiceController) addClusterIPJumpRule(chain, clusterIP, proto string, port int) error {
	if clusterIP == "" {
		return fmt.Errorf("clusterIP service requires clusterIP")
	}
	dst := clusterIP + "/32"
	cmd := c.commandFactory.Command(
		"iptables", "-t", "nat", "-A", "PREROUTING",
		"-d", dst,
		"-p", proto, "--dport", itoa(port),
		"-j", chain,
	)
	if err := cmd.Run(); err != nil {
		return err
	}
	cmd = c.commandFactory.Command(
		"iptables", "-t", "nat", "-A", "OUTPUT",
		"-d", dst,
		"-p", proto, "--dport", itoa(port),
		"-j", chain,
	)
	return cmd.Run()
}

func (c *ServiceController) addEndpointRule(chain, addr string, targetPort int, proto string, prob float64) error {
	args := []string{"-t", "nat", "-A", chain, "-p", proto}
	if prob > 0 {
		args = append(args, "-m", "statistic", "--mode", "random", "--probability", fmt.Sprintf("%.4f", prob))
	}
	args = append(args, "-j", "DNAT", "--to-destination", addr+":"+itoa(targetPort))
	cmd := c.commandFactory.Command("iptables", args...)
	return cmd.Run()
}

func (c *ServiceController) addForwardRules(chain string, ep svcEndpoint, targetPort int, proto string) error {
	if ep.HostInterface == "" || ep.Bridge == "" {
		return nil
	}
	hairpinCmd := []string{
		"-A", chain,
		"-i", ep.Bridge,
		"-o", ep.Bridge,
		"-p", proto,
		"-m", "conntrack",
		"--ctstate", "DNAT",
		"--dport", itoa(targetPort),
		"-d", ep.Addr,
		"-j", "ACCEPT",
	}
	_ = c.commandFactory.Command("iptables", hairpinCmd...).Run()

	inCmd := []string{
		"-A", chain,
		"-i", ep.HostInterface,
		"-o", ep.Bridge,
		"-p", proto,
		"--dport", itoa(targetPort),
		"-d", ep.Addr,
		"-j", "ACCEPT",
	}
	_ = c.commandFactory.Command("iptables", inCmd...).Run()

	outCmd := []string{
		"-A", chain,
		"-o", ep.HostInterface,
		"-i", ep.Bridge,
		"-p", proto,
		"--sport", itoa(targetPort),
		"-s", ep.Addr,
		"-j", "ACCEPT",
	}
	_ = c.commandFactory.Command("iptables", outCmd...).Run()
	return nil
}

type containerNotFound string

func (e containerNotFound) Error() string {
	return "infra container not found for pod: " + string(e)
}
