package promote

import (
	"raind/internal/raind/core/container"
	policycore "raind/internal/raind/core/policy"
	"sort"
	"strconv"
	"strings"
)

func AttachSecurityPoliciesFromPolicyList(draft *BottleDraft, inspects []container.ContainerInspectModel, policies []policycore.PolicyModel) {
	converted := ConvertSecurityPolicies(inspects, draft.Services, policies)
	draft.Policies = mergePolicyDrafts(draft.Policies, converted)
}

func ConvertSecurityPolicies(inspects []container.ContainerInspectModel, services []ServiceDraft, policies []policycore.PolicyModel) []PolicyDraft {
	endpointToService := buildPolicyEndpointMap(inspects, services)
	out := make([]PolicyDraft, 0)
	for _, p := range policies {
		if p.Status == "remove_next_commit" {
			continue
		}
		src := strings.TrimSpace(p.Source.ContainerName)
		dst := strings.TrimSpace(p.Destination.ContainerName)
		if src == "" || dst == "" {
			continue
		}
		srcSvc, ok := endpointToService[src]
		if !ok {
			continue
		}
		dstSvc, ok := endpointToService[dst]
		if !ok {
			continue
		}
		out = append(out, PolicyDraft{
			Type:        "east-west",
			Source:      srcSvc,
			Destination: dstSvc,
			Protocol:    strings.ToLower(strings.TrimSpace(p.Protocol)),
			DestPort:    p.DestPort,
			Comment:     strings.TrimSpace(p.Comment),
		})
	}
	sortPolicyDrafts(out)
	return out
}

func buildPolicyEndpointMap(inspects []container.ContainerInspectModel, services []ServiceDraft) map[string]string {
	out := map[string]string{}
	if len(inspects) == 1 && len(services) == 1 {
		addPolicyEndpoint(out, inspects[0].Name, services[0].Name)
		addPolicyEndpoint(out, inspects[0].ContainerId, services[0].Name)
		return out
	}
	serviceByName := map[string]string{}
	for _, svc := range services {
		serviceByName[svc.Name] = svc.Name
	}
	for _, inspect := range inspects {
		if svc, ok := serviceByName[sanitizeName(inspect.Name)]; ok {
			addPolicyEndpoint(out, inspect.Name, svc)
			addPolicyEndpoint(out, inspect.ContainerId, svc)
			continue
		}
		if svc, ok := serviceByName[sanitizeName(inspect.ContainerId)]; ok {
			addPolicyEndpoint(out, inspect.Name, svc)
			addPolicyEndpoint(out, inspect.ContainerId, svc)
		}
	}
	return out
}

func addPolicyEndpoint(m map[string]string, endpoint, service string) {
	endpoint = strings.TrimSpace(endpoint)
	service = strings.TrimSpace(service)
	if endpoint == "" || service == "" {
		return
	}
	m[endpoint] = service
}

func mergePolicyDrafts(existing, add []PolicyDraft) []PolicyDraft {
	if len(add) == 0 {
		return existing
	}
	seen := map[string]struct{}{}
	out := make([]PolicyDraft, 0, len(existing)+len(add))
	for _, p := range existing {
		key := policyDraftKey(p)
		seen[key] = struct{}{}
		out = append(out, p)
	}
	for _, p := range add {
		key := policyDraftKey(p)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	sortPolicyDrafts(out)
	return out
}

func policyDraftKey(p PolicyDraft) string {
	return strings.Join([]string{
		p.Type,
		p.Source,
		p.Destination,
		p.Protocol,
		strconv.Itoa(p.DestPort),
		p.Comment,
	}, "\x00")
}

func sortPolicyDrafts(policies []PolicyDraft) {
	sort.SliceStable(policies, func(i, j int) bool {
		a, b := policies[i], policies[j]
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		if a.Source != b.Source {
			return a.Source < b.Source
		}
		if a.Destination != b.Destination {
			return a.Destination < b.Destination
		}
		if a.Protocol != b.Protocol {
			return a.Protocol < b.Protocol
		}
		if a.DestPort != b.DestPort {
			return a.DestPort < b.DestPort
		}
		return a.Comment < b.Comment
	})
}
