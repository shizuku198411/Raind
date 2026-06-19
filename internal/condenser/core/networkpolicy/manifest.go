package networkpolicy

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

type Manifest struct {
	Name        string
	Namespace   string
	PodSelector map[string]string
	Ingress     []Rule
	Egress      []Rule
}

type Rule struct {
	Direction   string
	PodSelector map[string]string
	Protocol    string
	Port        int
}

type manifestMeta struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type k8sManifest struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Metadata   manifestMeta `yaml:"metadata"`
	Spec       k8sSpec      `yaml:"spec"`
}

type k8sSpec struct {
	PodSelector selector      `yaml:"podSelector"`
	Ingress     []ingressRule `yaml:"ingress"`
	Egress      []egressRule  `yaml:"egress"`
}

type selector struct {
	MatchLabels map[string]string `yaml:"matchLabels"`
}

type ingressRule struct {
	From  []peer `yaml:"from"`
	Ports []port `yaml:"ports"`
}

type egressRule struct {
	To    []peer `yaml:"to"`
	Ports []port `yaml:"ports"`
}

type peer struct {
	PodSelector       *selector `yaml:"podSelector"`
	NamespaceSelector *selector `yaml:"namespaceSelector"`
	IPBlock           any       `yaml:"ipBlock"`
}

type port struct {
	Protocol string `yaml:"protocol"`
	Port     int    `yaml:"port"`
}

func DecodeK8sNetworkPolicyManifest(body []byte) (Manifest, error) {
	dec := yaml.NewDecoder(bytes.NewReader(body))
	for {
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			return Manifest{}, err
		}
		if len(raw) == 0 {
			continue
		}
		kind, _ := raw["kind"].(string)
		if kind == "" {
			return Manifest{}, fmt.Errorf("kind is required")
		}
		if kind != "NetworkPolicy" {
			return Manifest{}, fmt.Errorf("unsupported kind: %s", kind)
		}
		rawBytes, err := yaml.Marshal(raw)
		if err != nil {
			return Manifest{}, err
		}
		var manifest k8sManifest
		if err := yaml.Unmarshal(rawBytes, &manifest); err != nil {
			return Manifest{}, err
		}
		return buildManifest(manifest)
	}
	return Manifest{}, fmt.Errorf("networkpolicy manifest not found")
}

func buildManifest(m k8sManifest) (Manifest, error) {
	if m.Metadata.Name == "" {
		return Manifest{}, fmt.Errorf("networkpolicy name is required")
	}
	if m.Metadata.Namespace == "" {
		m.Metadata.Namespace = "default"
	}
	result := Manifest{
		Name:        m.Metadata.Name,
		Namespace:   m.Metadata.Namespace,
		PodSelector: normalizeLabels(m.Spec.PodSelector.MatchLabels),
	}
	for _, rule := range m.Spec.Ingress {
		rules, err := expandIngressRule(rule)
		if err != nil {
			return Manifest{}, err
		}
		result.Ingress = append(result.Ingress, rules...)
	}
	for _, rule := range m.Spec.Egress {
		rules, err := expandEgressRule(rule)
		if err != nil {
			return Manifest{}, err
		}
		result.Egress = append(result.Egress, rules...)
	}
	return result, nil
}

func expandIngressRule(rule ingressRule) ([]Rule, error) {
	if len(rule.From) == 0 {
		return nil, fmt.Errorf("networkpolicy ingress.from with podSelector is required")
	}
	ports, err := normalizePorts(rule.Ports)
	if err != nil {
		return nil, err
	}
	var result []Rule
	for _, from := range rule.From {
		labels, err := peerPodSelector(from, "ingress.from")
		if err != nil {
			return nil, err
		}
		for _, port := range ports {
			result = append(result, Rule{
				Direction:   "ingress",
				PodSelector: labels,
				Protocol:    port.Protocol,
				Port:        port.Port,
			})
		}
	}
	return result, nil
}

func expandEgressRule(rule egressRule) ([]Rule, error) {
	if len(rule.To) == 0 {
		return nil, fmt.Errorf("networkpolicy egress.to with podSelector is required")
	}
	ports, err := normalizePorts(rule.Ports)
	if err != nil {
		return nil, err
	}
	var result []Rule
	for _, to := range rule.To {
		labels, err := peerPodSelector(to, "egress.to")
		if err != nil {
			return nil, err
		}
		for _, port := range ports {
			result = append(result, Rule{
				Direction:   "egress",
				PodSelector: labels,
				Protocol:    port.Protocol,
				Port:        port.Port,
			})
		}
	}
	return result, nil
}

func peerPodSelector(p peer, field string) (map[string]string, error) {
	if p.NamespaceSelector != nil {
		return nil, fmt.Errorf("%s.namespaceSelector is not supported yet", field)
	}
	if p.IPBlock != nil {
		return nil, fmt.Errorf("%s.ipBlock is not supported yet", field)
	}
	if p.PodSelector == nil {
		return nil, fmt.Errorf("%s.podSelector is required", field)
	}
	return normalizeLabels(p.PodSelector.MatchLabels), nil
}

func normalizePorts(ports []port) ([]port, error) {
	if len(ports) == 0 {
		return []port{{}}, nil
	}
	out := make([]port, 0, len(ports))
	for _, p := range ports {
		protocol := strings.ToLower(p.Protocol)
		if protocol == "" {
			protocol = "tcp"
		}
		if protocol != "tcp" && protocol != "udp" {
			return nil, fmt.Errorf("unsupported networkpolicy protocol: %s", p.Protocol)
		}
		out = append(out, port{Protocol: protocol, Port: p.Port})
	}
	return out, nil
}

func normalizeLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(labels))
	for key, value := range labels {
		out[key] = value
	}
	return out
}
