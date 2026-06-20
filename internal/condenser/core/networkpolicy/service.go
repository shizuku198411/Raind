package networkpolicy

import (
	"fmt"
	"sort"
	"strings"

	"raind/internal/condenser/store/csm"
	"raind/internal/condenser/store/netpol"
	"raind/internal/condenser/store/npm"
	"raind/internal/condenser/store/psm"
	"raind/internal/condenser/utils"
)

const networkPolicyOwnerKind = "NetworkPolicy"

func NewService() *Service {
	return &Service{
		psmHandler:    psm.NewPsmManager(psm.NewPsmStore(utils.PsmStorePath)),
		csmHandler:    csm.NewCsmManager(csm.NewCsmStore(utils.CsmStorePath)),
		npmHandler:    npm.NewNpmManager(npm.NewNpmStore(utils.NpmStorePath)),
		netpolHandler: netpol.NewManager(netpol.NewStore(utils.NetpolStorePath)),
	}
}

type Service struct {
	psmHandler    psm.PsmHandler
	csmHandler    csm.CsmHandler
	npmHandler    npm.NpmHandler
	netpolHandler netpol.Handler
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
	info := netpol.NetworkPolicyInfo{
		Name:             manifest.Name,
		Namespace:        manifest.Namespace,
		PodSelector:      cloneLabels(manifest.PodSelector),
		Ingress:          toRuleInfo(manifest.Ingress),
		Egress:           toRuleInfo(manifest.Egress),
		GeneratedRuleIds: nil,
	}
	if err := s.netpolHandler.StoreNetworkPolicy(networkPolicyId, info); err != nil {
		return netpol.NetworkPolicyInfo{}, err
	}

	if _, err := s.ReconcileAll(); err != nil {
		_ = s.netpolHandler.RemoveNetworkPolicy(networkPolicyId)
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
	if err := s.netpolHandler.RemoveNetworkPolicy(info.NetworkPolicyId); err != nil {
		return netpol.NetworkPolicyInfo{}, err
	}
	return info, nil
}

func (s *Service) ReconcileAll() (bool, error) {
	infos, err := s.netpolHandler.GetNetworkPolicyList()
	if err != nil {
		return false, err
	}
	sortNetworkPolicies(infos)

	desired := make([]npm.Policy, 0)
	generatedByPolicy := map[string][]string{}
	for _, info := range infos {
		manifest := manifestFromInfo(info)
		generated, err := s.generateBackendPolicies(info.NetworkPolicyId, manifest)
		if err != nil {
			return false, err
		}
		desired = append(desired, generated...)
		for _, policy := range generated {
			generatedByPolicy[info.NetworkPolicyId] = append(generatedByPolicy[info.NetworkPolicyId], policy.Id)
		}
	}
	sortPolicies(desired)

	current := filterPoliciesByOwnerKind(s.npmHandler.GetEWPolicyList(), networkPolicyOwnerKind)
	sortPolicies(current)
	if equalPolicySets(current, desired) {
		return false, nil
	}

	if err := s.npmHandler.ReplacePoliciesByOwnerKind("RAIND-EW", networkPolicyOwnerKind, desired); err != nil {
		return false, err
	}
	for _, info := range infos {
		ids := generatedByPolicy[info.NetworkPolicyId]
		sort.Strings(ids)
		if err := s.netpolHandler.UpdateGeneratedRuleIds(info.NetworkPolicyId, ids); err != nil {
			return false, err
		}
	}
	return true, nil
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
	sortPolicies(policies)
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
		if pod.Namespace != namespace || !labelsMatch(pod.Labels, labels) {
			continue
		}
		infra, ok, err := s.readyInfraContainer(pod)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		result = append(result, podEndpoint{pod: pod, container: infra})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].pod.Namespace != result[j].pod.Namespace {
			return result[i].pod.Namespace < result[j].pod.Namespace
		}
		if result[i].pod.Name != result[j].pod.Name {
			return result[i].pod.Name < result[j].pod.Name
		}
		return result[i].container.ContainerName < result[j].container.ContainerName
	})
	return result, nil
}

func (s *Service) readyInfraContainer(pod psm.PodInfo) (csm.ContainerInfo, bool, error) {
	if pod.State != psm.PodStateRunning || pod.StoppedByUser {
		return csm.ContainerInfo{}, false, nil
	}
	containers, err := s.csmHandler.GetContainersByPodId(pod.PodId)
	if err != nil {
		return csm.ContainerInfo{}, false, err
	}
	if len(containers) == 0 {
		return csm.ContainerInfo{}, false, nil
	}

	var (
		infra         csm.ContainerInfo
		runningByName = map[string]struct{}{}
		memberCount   int
	)
	for _, container := range containers {
		if strings.HasPrefix(container.ContainerName, utils.PodInfraContainerNamePrefix) {
			if container.State != psm.ContainerStateRunning {
				return csm.ContainerInfo{}, false, nil
			}
			infra = container
			continue
		}
		memberCount++
		if container.State != psm.ContainerStateRunning {
			return csm.ContainerInfo{}, false, nil
		}
		runningByName[container.ContainerName] = struct{}{}
	}
	if infra.ContainerId == "" || memberCount == 0 {
		return csm.ContainerInfo{}, false, nil
	}
	if !s.expectedMembersRunning(pod, runningByName) {
		return csm.ContainerInfo{}, false, nil
	}
	return infra, true, nil
}

func (s *Service) expectedMembersRunning(pod psm.PodInfo, runningByName map[string]struct{}) bool {
	if pod.TemplateId == "" {
		return true
	}
	tpl, err := s.psmHandler.GetPodTemplate(pod.TemplateId)
	if err != nil {
		return false
	}
	for _, spec := range tpl.Spec.Containers {
		if spec.Name == "" {
			continue
		}
		if _, ok := runningByName[buildPodMemberName(spec.Name, pod.PodId)]; !ok {
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
		Source:      npm.HostInfo{ContainerName: src.container.ContainerName, DisplayName: src.pod.Name},
		Destination: npm.HostInfo{ContainerName: dst.container.ContainerName, DisplayName: dst.pod.Name},
		Protocol:    rule.Protocol,
		DestPort:    rule.Port,
		Comment:     fmt.Sprintf("NetworkPolicy %s/%s %s %s->%s", manifest.Namespace, manifest.Name, rule.Direction, src.pod.Name, dst.pod.Name),
		ManagedBy:   "resource",
		OwnerKind:   networkPolicyOwnerKind,
		OwnerNS:     manifest.Namespace,
		OwnerName:   manifest.Name,
		OwnerId:     networkPolicyId,
	}
}

func manifestFromInfo(info netpol.NetworkPolicyInfo) Manifest {
	return Manifest{
		Name:        info.Name,
		Namespace:   info.Namespace,
		PodSelector: cloneLabels(info.PodSelector),
		Ingress:     rulesFromInfo(info.Ingress),
		Egress:      rulesFromInfo(info.Egress),
	}
}

func rulesFromInfo(rules []netpol.RuleInfo) []Rule {
	out := make([]Rule, 0, len(rules))
	for _, rule := range rules {
		out = append(out, Rule{
			Direction:   rule.Direction,
			PodSelector: cloneLabels(rule.PodSelector),
			Protocol:    rule.Protocol,
			Port:        rule.Port,
		})
	}
	return out
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

func filterPoliciesByOwnerKind(policies []npm.Policy, ownerKind string) []npm.Policy {
	filtered := make([]npm.Policy, 0, len(policies))
	for _, policy := range policies {
		if policy.OwnerKind == ownerKind {
			filtered = append(filtered, policy)
		}
	}
	return filtered
}

func sortPolicies(policies []npm.Policy) {
	sort.Slice(policies, func(i, j int) bool {
		return policyStateKey(policies[i]) < policyStateKey(policies[j])
	})
}

func equalPolicySets(a, b []npm.Policy) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if policyStateKey(a[i]) != policyStateKey(b[i]) {
			return false
		}
	}
	return true
}

func policyStateKey(policy npm.Policy) string {
	return strings.Join([]string{
		policy.OwnerKind,
		policy.OwnerId,
		policy.OwnerNS,
		policy.OwnerName,
		policy.Source.ContainerName,
		policy.Source.DisplayName,
		policy.Destination.ContainerName,
		policy.Destination.DisplayName,
		policy.Protocol,
		fmt.Sprint(policy.DestPort),
		policy.Comment,
		policy.ManagedBy,
	}, "\x00")
}
