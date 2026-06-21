package promote

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type BottleToResourcesOptions struct {
	Namespace   string
	IngressHost string
}

type bottleSourceSpec struct {
	Bottle   bottleSourceMeta               `yaml:"bottle"`
	Services map[string]bottleSourceService `yaml:"services"`
	Policies []bottleSourcePolicy           `yaml:"policies,omitempty"`
}

type bottleSourceMeta struct {
	Name string `yaml:"name"`
}

type bottleSourceService struct {
	Image      string   `yaml:"image"`
	Command    []string `yaml:"command,omitempty"`
	Env        []string `yaml:"env,omitempty"`
	Ports      []string `yaml:"ports,omitempty"`
	Mount      []string `yaml:"mount,omitempty"`
	CapAdd     []string `yaml:"capAdd,omitempty"`
	CapAddAlt  []string `yaml:"cap-add,omitempty"`
	CapDrop    []string `yaml:"capDrop,omitempty"`
	CapDropAlt []string `yaml:"cap-drop,omitempty"`
	Network    string   `yaml:"network,omitempty"`
	Tty        bool     `yaml:"tty,omitempty"`
	DependsOn  []string `yaml:"depends_on,omitempty"`
}

type bottleSourcePolicy struct {
	Type        string `yaml:"type"`
	Source      string `yaml:"source"`
	Destination string `yaml:"destination"`
	Protocol    string `yaml:"protocol,omitempty"`
	DestPort    int    `yaml:"dest_port,omitempty"`
	Comment     string `yaml:"comment,omitempty"`
}

func BuildResourceDraftFromBottleFile(path string, opt BottleToResourcesOptions) (BottleDraft, error) {
	body, err := readBottleSourceFile(path)
	if err != nil {
		return BottleDraft{}, err
	}
	return BuildResourceDraftFromBottle(body, opt)
}

func readBottleSourceFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func BuildResourceDraftFromBottle(body []byte, opt BottleToResourcesOptions) (BottleDraft, error) {
	spec, err := parseBottleSource(body)
	if err != nil {
		return BottleDraft{}, err
	}
	if strings.TrimSpace(spec.Bottle.Name) == "" {
		return BottleDraft{}, fmt.Errorf("bottle.name is required")
	}
	if len(spec.Services) == 0 {
		return BottleDraft{}, fmt.Errorf("services is required")
	}

	services := make([]ServiceDraft, 0, len(spec.Services))
	for name, svc := range spec.Services {
		serviceName := sanitizeName(name)
		if serviceName == "" {
			return BottleDraft{}, fmt.Errorf("service name %q is not valid for generated resources", name)
		}
		services = append(services, ServiceDraft{
			Name:      serviceName,
			Image:     strings.TrimSpace(svc.Image),
			Command:   cleanStrings(svc.Command),
			Env:       envFromBottleStrings(svc.Env),
			Ports:     portsFromBottleStrings(svc.Ports),
			Mounts:    mountsFromBottleStrings(svc.Mount),
			CapAdd:    mergeUniqueStrings(svc.CapAdd, svc.CapAddAlt),
			CapDrop:   mergeUniqueStrings(svc.CapDrop, svc.CapDropAlt),
			Network:   strings.TrimSpace(svc.Network),
			Tty:       svc.Tty,
			DependsOn: cleanStrings(svc.DependsOn),
		})
	}
	sort.SliceStable(services, func(i, j int) bool { return services[i].Name < services[j].Name })

	policies := make([]PolicyDraft, 0, len(spec.Policies))
	for _, policy := range spec.Policies {
		p := PolicyDraft{
			Type:        strings.TrimSpace(policy.Type),
			Source:      sanitizeName(policy.Source),
			Destination: sanitizeName(policy.Destination),
			Protocol:    strings.ToLower(strings.TrimSpace(policy.Protocol)),
			DestPort:    policy.DestPort,
			Comment:     strings.TrimSpace(policy.Comment),
		}
		if p.Type == "" {
			p.Type = "east-west"
		}
		if p.Protocol == "" {
			p.Protocol = "tcp"
		}
		if p.Source == "" || p.Destination == "" {
			continue
		}
		policies = append(policies, p)
	}
	sort.SliceStable(policies, func(i, j int) bool {
		if policies[i].Source != policies[j].Source {
			return policies[i].Source < policies[j].Source
		}
		if policies[i].Destination != policies[j].Destination {
			return policies[i].Destination < policies[j].Destination
		}
		return policies[i].DestPort < policies[j].DestPort
	})

	bottleName := sanitizeName(spec.Bottle.Name)
	if bottleName == "" {
		bottleName = "app"
	}
	namespace := sanitizeName(opt.Namespace)
	if namespace == "" {
		namespace = bottleName
	}

	return BottleDraft{
		SourceContainer: "bottle/" + spec.Bottle.Name,
		BottleName:      namespace,
		Services:        services,
		Policies:        policies,
		IngressHost:     strings.TrimSpace(opt.IngressHost),
	}, nil
}

func parseBottleSource(body []byte) (bottleSourceSpec, error) {
	var spec bottleSourceSpec
	if err := yaml.Unmarshal(body, &spec); err != nil {
		return bottleSourceSpec{}, err
	}
	return spec, nil
}

func envFromBottleStrings(values []string) []EnvVar {
	out := make([]EnvVar, 0, len(values))
	for _, raw := range values {
		key, value, ok := strings.Cut(raw, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			continue
		}
		out = append(out, EnvVar{Key: key, Value: value, Sensitive: IsSecretLikeKey(key)})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func portsFromBottleStrings(values []string) []PortMapping {
	out := make([]PortMapping, 0, len(values))
	for _, raw := range values {
		p, ok := parseBottlePort(raw)
		if ok {
			out = append(out, p)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ContainerPort != out[j].ContainerPort {
			return out[i].ContainerPort < out[j].ContainerPort
		}
		if out[i].HostPort != out[j].HostPort {
			return out[i].HostPort < out[j].HostPort
		}
		return out[i].Protocol < out[j].Protocol
	})
	return out
}

func parseBottlePort(raw string) (PortMapping, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) == 0 || len(parts) > 3 {
		return PortMapping{}, false
	}
	protocol := "tcp"
	if len(parts) == 3 {
		protocol = strings.ToLower(strings.TrimSpace(parts[2]))
	}
	containerPart := parts[len(parts)-1]
	if len(parts) == 3 {
		containerPart = parts[1]
	}
	containerPort, ok := atoiPositive(containerPart)
	if !ok {
		return PortMapping{}, false
	}
	hostPort := containerPort
	if len(parts) >= 2 {
		if parsed, ok := atoiPositive(parts[0]); ok {
			hostPort = parsed
		}
	}
	return PortMapping{HostPort: hostPort, ContainerPort: containerPort, Protocol: protocol}, true
}

func mountsFromBottleStrings(values []string) []MountMapping {
	out := make([]MountMapping, 0, len(values))
	for _, raw := range values {
		m, ok := parseBottleMount(raw)
		if ok {
			out = append(out, m)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Destination < out[j].Destination })
	return out
}

func parseBottleMount(raw string) (MountMapping, bool) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) < 2 {
		return MountMapping{}, false
	}
	m := MountMapping{Source: strings.TrimSpace(parts[0]), Destination: strings.TrimSpace(parts[1])}
	if m.Source == "" || m.Destination == "" {
		return MountMapping{}, false
	}
	for _, opt := range parts[2:] {
		opt = strings.TrimSpace(opt)
		if opt == "" {
			continue
		}
		if opt == "ro" {
			m.ReadOnly = true
		}
		m.Options = append(m.Options, opt)
	}
	return m, true
}

func atoiPositive(raw string) (int, bool) {
	var n int
	for _, r := range strings.TrimSpace(raw) {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, n > 0
}
