package service

import (
	"condenser/internal/core/container"
	"condenser/internal/store/ipam"
	"condenser/internal/store/psm"
	"condenser/internal/store/ssm"
	"condenser/internal/utils"
	"fmt"
	"log"
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
		lastPorts:        map[string][]ssm.ServicePort{},
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
	lastPorts        map[string][]ssm.ServicePort
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
				c.lastState[svc.ServiceId] = stateKey
				c.lastPorts[svc.ServiceId] = svc.Ports
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
			delete(c.lastPorts, serviceId)
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
		infraId, err := c.getInfraContainerId(p.PodId)
		if err != nil {
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
		_ = c.deleteJumpRule(chain, proto, port.Port)
		if err := c.addJumpRule(chain, proto, port.Port); err != nil {
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
			_ = c.addForwardRules(ep, port.TargetPort, proto)
		}
	}
	return nil
}

func (c *ServiceController) serviceChainName(serviceId string, port int) string {
	id := serviceId
	if len(id) > 8 {
		id = id[:8]
	}
	return "RAIND-SVC-" + id + "-" + itoa(port)
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
	ports := c.lastPorts[serviceId]
	if len(ports) == 0 {
		return
	}
	for _, p := range ports {
		if p.Port == 0 {
			continue
		}
		chain := c.serviceChainName(serviceId, p.Port)
		_ = c.deleteJumpRule(chain, "tcp", p.Port)
		_ = c.deleteJumpRule(chain, "udp", p.Port)
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

func (c *ServiceController) deleteJumpRule(chain, proto string, port int) error {
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

func (c *ServiceController) addJumpRule(chain, proto string, port int) error {
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

func (c *ServiceController) addEndpointRule(chain, addr string, targetPort int, proto string, prob float64) error {
	args := []string{"-t", "nat", "-A", chain, "-p", proto}
	if prob > 0 {
		args = append(args, "-m", "statistic", "--mode", "random", "--probability", fmt.Sprintf("%.4f", prob))
	}
	args = append(args, "-j", "DNAT", "--to-destination", addr+":"+itoa(targetPort))
	cmd := c.commandFactory.Command("iptables", args...)
	return cmd.Run()
}

func (c *ServiceController) addForwardRules(ep svcEndpoint, targetPort int, proto string) error {
	if ep.HostInterface == "" || ep.Bridge == "" {
		return nil
	}
	hairpinCmd := []string{
		"-I", "FORWARD", "1",
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
		"-A", "FORWARD",
		"-i", ep.HostInterface,
		"-o", ep.Bridge,
		"-p", proto,
		"--dport", itoa(targetPort),
		"-d", ep.Addr,
		"-j", "ACCEPT",
	}
	_ = c.commandFactory.Command("iptables", inCmd...).Run()

	outCmd := []string{
		"-A", "FORWARD",
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
