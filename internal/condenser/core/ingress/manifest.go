package ingress

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"raind/internal/condenser/store/ism"

	"gopkg.in/yaml.v3"
)

type IngressManifest struct {
	Name      string
	Namespace string
	Rules     []ism.IngressRule
	TLSHosts  []string
}

type ingressMeta struct {
	Name      string            `yaml:"name"`
	Namespace string            `yaml:"namespace"`
	Labels    map[string]string `yaml:"labels"`
}

type ingressManifest struct {
	APIVersion string      `yaml:"apiVersion"`
	Kind       string      `yaml:"kind"`
	Metadata   ingressMeta `yaml:"metadata"`
	Spec       ingressSpec `yaml:"spec"`
}

type ingressSpec struct {
	TLS   []ingressTLSManifest  `yaml:"tls"`
	Rules []ingressRuleManifest `yaml:"rules"`
}

type ingressTLSManifest struct {
	Hosts      []string `yaml:"hosts"`
	SecretName string   `yaml:"secretName"`
}

type ingressRuleManifest struct {
	Host string              `yaml:"host"`
	HTTP ingressHTTPManifest `yaml:"http"`
}

type ingressHTTPManifest struct {
	Paths []ingressPathManifest `yaml:"paths"`
}

type ingressPathManifest struct {
	Path     string                 `yaml:"path"`
	PathType string                 `yaml:"pathType"`
	Backend  ingressBackendManifest `yaml:"backend"`
}

type ingressBackendManifest struct {
	Service ingressBackendServiceManifest `yaml:"service"`
}

type ingressBackendServiceManifest struct {
	Name string                     `yaml:"name"`
	Port ingressBackendPortManifest `yaml:"port"`
}

type ingressBackendPortManifest struct {
	Number int    `yaml:"number"`
	Name   string `yaml:"name"`
}

func DecodeK8sIngressManifest(body []byte) (IngressManifest, error) {
	dec := yaml.NewDecoder(bytes.NewReader(body))
	for {
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			return IngressManifest{}, err
		}
		if len(raw) == 0 {
			continue
		}
		kind, _ := raw["kind"].(string)
		if kind == "" {
			return IngressManifest{}, fmt.Errorf("kind is required")
		}
		if kind != "Ingress" {
			return IngressManifest{}, fmt.Errorf("unsupported kind: %s", kind)
		}
		rawBytes, err := yaml.Marshal(raw)
		if err != nil {
			return IngressManifest{}, err
		}
		var in ingressManifest
		if err := yaml.Unmarshal(rawBytes, &in); err != nil {
			return IngressManifest{}, err
		}
		return buildIngressManifest(in)
	}
	return IngressManifest{}, fmt.Errorf("ingress manifest not found")
}

func buildIngressManifest(in ingressManifest) (IngressManifest, error) {
	if in.Metadata.Namespace == "" {
		in.Metadata.Namespace = "default"
	}
	if in.Metadata.Name == "" {
		return IngressManifest{}, fmt.Errorf("ingress name is required")
	}

	rules := make([]ism.IngressRule, 0, len(in.Spec.Rules))
	for _, r := range in.Spec.Rules {
		paths := make([]ism.IngressPath, 0, len(r.HTTP.Paths))
		for _, p := range r.HTTP.Paths {
			path := p.Path
			if path == "" {
				path = "/"
			}
			if !strings.HasPrefix(path, "/") {
				return IngressManifest{}, fmt.Errorf("ingress path must start with /: %s", path)
			}

			pathType := p.PathType
			if pathType == "" {
				pathType = "Prefix"
			}
			switch pathType {
			case "Prefix", "Exact":
			default:
				return IngressManifest{}, fmt.Errorf("unsupported ingress pathType: %s", pathType)
			}

			if p.Backend.Service.Name == "" {
				return IngressManifest{}, fmt.Errorf("ingress backend service name is required")
			}
			if p.Backend.Service.Port.Number == 0 {
				return IngressManifest{}, fmt.Errorf("ingress backend service port.number is required")
			}

			paths = append(paths, ism.IngressPath{
				Path:     path,
				PathType: pathType,
				Backend: ism.IngressBackend{
					ServiceName: p.Backend.Service.Name,
					ServicePort: p.Backend.Service.Port.Number,
				},
			})
		}
		if len(paths) == 0 {
			return IngressManifest{}, fmt.Errorf("ingress rule must have at least one http path")
		}
		rules = append(rules, ism.IngressRule{Host: strings.ToLower(strings.TrimSpace(r.Host)), Paths: paths})
	}
	if len(rules) == 0 {
		return IngressManifest{}, fmt.Errorf("ingress must have at least one rule")
	}
	tlsHosts, err := buildTLSHosts(in.Spec.TLS)
	if err != nil {
		return IngressManifest{}, err
	}

	return IngressManifest{Name: in.Metadata.Name, Namespace: in.Metadata.Namespace, Rules: rules, TLSHosts: tlsHosts}, nil
}

func buildTLSHosts(entries []ingressTLSManifest) ([]string, error) {
	seen := map[string]bool{}
	var hosts []string
	for _, entry := range entries {
		for _, rawHost := range entry.Hosts {
			host := strings.ToLower(strings.TrimSpace(rawHost))
			if host == "" {
				continue
			}
			if strings.Contains(host, "*") {
				return nil, fmt.Errorf("ingress tls wildcard host is not supported yet: %s", host)
			}
			if strings.Contains(host, ":") {
				return nil, fmt.Errorf("ingress tls host must not include port: %s", host)
			}
			if !seen[host] {
				seen[host] = true
				hosts = append(hosts, host)
			}
		}
	}
	return hosts, nil
}
