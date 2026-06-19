package networkpolicy

import (
	"fmt"
	"sort"
	"strings"

	"raind/internal/condenser/core/policy"
	"raind/internal/condenser/store/csm"
	"raind/internal/condenser/store/netpol"
	"raind/internal/condenser/store/npm"
	"raind/internal/condenser/store/psm"
	"raind/internal/condenser/utils"
)

func NewService() *Service {
	return &Service{
		psmHandler:    psm.NewPsmManager(psm.NewPsmStore(utils.PsmStorePath)),
		csmHandler:    csm.NewCsmManager(csm.NewCsmStore(utils.CsmStorePath)),
		npmHandler:    npm.NewNpmManager(npm.NewNpmStore(utils.NpmStorePath)),
		netpolHandler: netpol.NewManager(netpol.NewStore(utils.NetpolStorePath)),
		policyHandler: policy.NewwServicePolicy(),
	}
}

type Service struct {
	psmHandler    psm.PsmHandler
	csmHandler    csm.CsmHandler
	npmHandler    npm.NpmHandler
	netpolHandler netpol.Handler
	policyHandler policy.PolicyServiceHandler
}

func (s *Service) Apply(manifest Manifest) (netpol.NetworkPolicyInfo, error) {
	if manifest.Name == "" {
		return netpol.NetworkPolicyInfo{}, fmt.Errorf("networkpolicy name is required")
	}
	if manifest.Namespace == "" {
		manifest.Namespace = "default"
	}
	if s.netpolHandler.IsNameAlreadyUsed(manifest.Name, manifest.Namespace) {
		return netpol.NetworkPolicyInfo{}, fmt.Errorf("name already used by other networkpolicy")
	}

	networkPolicyId := utils.NewUlid()
	generated, err := s.generateBackendPolicies(networkPolicyId, manifest)
	if err != nil {
		return netpol.NetworkPolicyInfo{}, err
	}
	generatedIds := make([]string, 0, len(generated))
	for _, backendPolicy := range generated {
		if err := s.npmHandler.AddPolicy("RAIND-EW", backendPolicy); err != nil {
			_, _ = s.npmHandler.RemovePoliciesByOwner("NetworkPolicy", networkPolicyId)
			return netpol.NetworkPolicyInfo{}, err
		}
		generatedIds = append(generatedIds, backendPolicy.Id)
	}

	info := netpol.NetworkPolicyInfo{
		Name:             manifest.Name,
		Namespace:        manifest.Namespace,
		PodSelector:      cloneLabels(manifest.PodSelector),
		Ingress:          toRuleInfo(manifest.Ingress),
		Egress:           toRuleInfo(manifest.Egress),
		GeneratedRuleIds: generatedIds,
	}
	if err := s.netpolHandler.StoreNetworkPolicy(networkPolicyId, info); err != nil {
		_, _ = s.npmHandler.RemovePoliciesByOwner("NetworkPolicy", networkPolicyId)
		return netpol.NetworkPolicyInfo{}, err
	}
	if err := s.policyHandler.CommitPolicy(); err != nil {
		_ = s.netpolHandler.RemoveNetworkPolicy(networkPolicyId)
		_, _ = s.npmHandler.RemovePoliciesByOwner("NetworkPolicy", networkPolicyId)
		return netpol.NetworkPolicyInfo{}, err
	}
	return s.netpolHandler.GetNetworkPolicyById(networkPolicyId)
}

func (s *Service) List(namespace string) ([]netpol.NetworkPolicyInfo, error) {
	list, err := s.netpolHandler.GetNetworkPolicyList()
	if err != nil {
		return nil, err
	}
	if namespace == "" {
		sortNetworkPolicies(list)
		return list, nil
	}
	filtered := make([]netpol.NetworkPolicyInfo, 0, len(list))
	for _, info := range list {
		if info.Namespace == namespace {
			filtered = append(filtered, info)
		}
	}
	sortNetworkPolicies(filtered)
	return filtered, nil
}

func (s *Service) Get(idOrName, namespace string) (netpol.NetworkPolicyInfo, error) {
	if namespace != "" {
		return s.netpolHandler.GetNetworkPolicyByName(idOrName, namespace)
	}
	if info, err := s.netpolHandler.GetNetworkPolicyById(idOrName); err == nil {
		return info, nil
	}
	list, err := s.netpolHandler.GetNetworkPolicyList()
	if err != nil {
		return netpol.NetworkPolicyInfo{}, err
	}
	var found []netpol.NetworkPolicyInfo
	for _, info := range list {
		if info.Name == idOrName {
			found = append(found, info)
		}
	}
	if len(found) == 1 {
		return found[0], nil
	}
	if len(found) > 1 {
		return netpol.NetworkPolicyInfo{}, fmt.Errorf("networkpolicy name %q exists in multiple namespaces; specify namespace", idOrName)
	}
	return netpol.NetworkPolicyInfo{}, fmt.Errorf("networkpolicy %q not found", idOrName)
}

func (s *Service) Remove(idOrName, namespace string) (netpol.NetworkPolicyInfo, error) {
	info, err := s.Get(idOrName, namespace)
	if err != nil {
		return netpol.NetworkPolicyInfo{}, err
	}
	if _, err := s.npmHandler.RemovePoliciesByOwner("NetworkPolicy", info.NetworkPolicyId); err != nil {
		return netpol.NetworkPolicyInfo{}, err
	}
	if err := s.netpolHandler.RemoveNetworkPolicy(info.NetworkPolicyId); err != nil {
		return netpol.NetworkPolicyInfo{}, err
	}
	if err := s.policyHandler.CommitPolicy(); err != nil {
		return netpol.NetworkPolicyInfo{}, err
	}
	return info, nil
}

func (s *Service) generateBackendPolicies(networkPolicyId string, manifest Manifest) ([]npm.Policy, error) {
	selectedPods, err := s.selectPods(manifest.Namespace, manifest.PodSelector)
	if err != nil {
		return nil, err
	}
	policySet := map[string]npm.Policy{}
	for _, rule := range manifest.Ingress {
		sourcePods, err := s.selectPods(manifest.Namespace, rule.PodSelector)
		if err != nil {
			return nil, err
		}
		for _, dst := range selectedPods {
			for _, src := range sourcePods {
				s.addBackendPolicy(policySet, networkPolicyId, manifest, rule, src, dst)
			}
		}
	}
	for _, rule := range manifest.Egress {
		destinationPods, err := s.selectPods(manifest.Namespace, rule.PodSelector)
		if err != nil {
			return nil, err
		}
		for _, src := range selectedPods {
			for _, dst := range destinationPods {
				s.addBackendPolicy(policySet, networkPolicyId, manifest, rule, src, dst)
			}
		}
	}
	policies := make([]npm.Policy, 0, len(policySet))
	for _, backendPolicy := range policySet {
		policies = append(policies, backendPolicy)
	}
	sort.Slice(policies, func(i, j int) bool { return policies[i].Id < policies[j].Id })
	return policies, nil
}

type podEndpoint struct {
	pod       psm.PodInfo
	container csm.ContainerInfo
}

func (s *Service) selectPods(namespace string, labels map[string]string) ([]podEndpoint, error) {
	pods, err := s.psmHandler.GetPodList()
	if err != nil {
		return nil, err
	}
	var result []podEndpoint
	for _, pod := range pods {
		if pod.Namespace != namespace || pod.State != psm.PodStateRunning || !labelsMatch(pod.Labels, labels) {
			continue
		}
		infra, ok, err := s.runningInfraContainer(pod.PodId)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		result = append(result, podEndpoint{pod: pod, container: infra})
	}
	return result, nil
}

func (s *Service) runningInfraContainer(podId string) (csm.ContainerInfo, bool, error) {
	containers, err := s.csmHandler.GetContainersByPodId(podId)
	if err != nil {
		return csm.ContainerInfo{}, false, err
	}
	for _, container := range containers {
		if container.State == psm.ContainerStateRunning && strings.HasPrefix(container.ContainerName, utils.PodInfraContainerNamePrefix) {
			return container, true, nil
		}
	}
	return csm.ContainerInfo{}, false, nil
}

func (s *Service) addBackendPolicy(policySet map[string]npm.Policy, networkPolicyId string, manifest Manifest, rule Rule, src, dst podEndpoint) {
	if src.container.ContainerId == dst.container.ContainerId {
		return
	}
	key := strings.Join([]string{src.container.ContainerName, dst.container.ContainerName, rule.Protocol, fmt.Sprint(rule.Port)}, "\x00")
	if _, ok := policySet[key]; ok {
		return
	}
	policySet[key] = npm.Policy{
		Id:          utils.NewUlid(),
		Status:      "before_commit",
		Source:      npm.HostInfo{ContainerName: src.container.ContainerName},
		Destination: npm.HostInfo{ContainerName: dst.container.ContainerName},
		Protocol:    rule.Protocol,
		DestPort:    rule.Port,
		Comment:     fmt.Sprintf("NetworkPolicy %s/%s %s %s->%s", manifest.Namespace, manifest.Name, rule.Direction, src.pod.Name, dst.pod.Name),
		ManagedBy:   "resource",
		OwnerKind:   "NetworkPolicy",
		OwnerNS:     manifest.Namespace,
		OwnerName:   manifest.Name,
		OwnerId:     networkPolicyId,
	}
}

func labelsMatch(labels, selector map[string]string) bool {
	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
}

func toRuleInfo(rules []Rule) []netpol.RuleInfo {
	out := make([]netpol.RuleInfo, 0, len(rules))
	for _, rule := range rules {
		out = append(out, netpol.RuleInfo{
			Direction:   rule.Direction,
			PodSelector: cloneLabels(rule.PodSelector),
			Protocol:    rule.Protocol,
			Port:        rule.Port,
		})
	}
	return out
}

func cloneLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(labels))
	for key, value := range labels {
		out[key] = value
	}
	return out
}

func sortNetworkPolicies(list []netpol.NetworkPolicyInfo) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].Namespace != list[j].Namespace {
			return list[i].Namespace < list[j].Namespace
		}
		return list[i].Name < list[j].Name
	})
}
