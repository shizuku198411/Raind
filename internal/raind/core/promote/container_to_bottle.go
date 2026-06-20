package promote

import (
	"fmt"
	"path/filepath"
	"raind/internal/raind/core/container"
	"sort"
	"strings"
)

func BuildBottleDraftFromContainer(inspect container.ContainerInspectModel, opt ContainerToBottleOptions) (BottleDraft, error) {
	draft, err := BuildBottleDraftFromContainers([]container.ContainerInspectModel{inspect}, opt)
	if err != nil {
		return BottleDraft{}, err
	}
	return draft, nil
}

func BuildBottleDraftFromContainers(inspects []container.ContainerInspectModel, opt ContainerToBottleOptions) (BottleDraft, error) {
	if len(inspects) == 0 {
		return BottleDraft{}, fmt.Errorf("at least one container is required")
	}
	if len(inspects) > 1 && strings.TrimSpace(opt.ServiceName) != "" {
		return BottleDraft{}, fmt.Errorf("--service-name can only be used when promoting a single container")
	}

	services := make([]ServiceDraft, 0, len(inspects))
	warnings := []Warning{}
	seenServices := map[string]struct{}{}
	sourceRefs := make([]string, 0, len(inspects))

	for _, inspect := range inspects {
		if inspect.PodId != "" && !opt.AllowPodContainer {
			return BottleDraft{}, fmt.Errorf("container %s is a pod member; promoting pod member containers is not supported yet", inspect.Name)
		}

		serviceName := serviceNameForInspect(inspect, opt, len(inspects) == 1)
		if _, exists := seenServices[serviceName]; exists {
			return BottleDraft{}, fmt.Errorf("multiple containers map to service name %q; rename one container or promote them separately", serviceName)
		}
		seenServices[serviceName] = struct{}{}

		svc := ServiceDraft{
			Name:    serviceName,
			Image:   formatImage(inspect.ImageRepository, inspect.ImageReference),
			Command: commandFromInspect(inspect),
			Env:     envFromInspect(inspect, opt.IncludeImageEnv),
			Ports:   portsFromInspect(inspect),
			Mounts:  mountsFromInspect(inspect),
			Tty:     inspect.Tty,
		}
		if svc.Image == "" {
			warnings = append(warnings, Warning{Code: "missing-image", Message: fmt.Sprintf("container %s image could not be determined from inspect data", displayContainerRef(inspect))})
		}
		if inspect.SecurityProfile != "" && inspect.SecurityProfile != "default" {
			warnings = append(warnings, Warning{Code: "security-profile", Message: fmt.Sprintf("container %s used security profile %q; bottle.yaml draft does not currently preserve security profiles", displayContainerRef(inspect), inspect.SecurityProfile)})
		}
		services = append(services, svc)
		sourceRefs = append(sourceRefs, displayContainerRef(inspect))
	}

	sort.SliceStable(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	inferServiceDependencies(services)

	bottleName := strings.TrimSpace(opt.BottleName)
	if bottleName == "" {
		if len(services) == 1 {
			bottleName = sanitizeName(inspects[0].Name)
			if bottleName == "" {
				bottleName = sanitizeName(inspects[0].ContainerId)
			}
		} else {
			bottleName = "promoted-bottle"
		}
	}
	if bottleName == "" {
		bottleName = "app"
	}

	d := BottleDraft{
		SourceContainer: strings.Join(sourceRefs, ", "),
		BottleName:      bottleName,
		Services:        services,
		Warnings:        warnings,
	}

	// Populate legacy single-service fields for existing callers/tests.
	if len(services) == 1 {
		svc := services[0]
		d.ServiceName = svc.Name
		d.Image = svc.Image
		d.Command = svc.Command
		d.Env = svc.Env
		d.Ports = svc.Ports
		d.Mounts = svc.Mounts
		d.Network = svc.Network
		d.Tty = svc.Tty
	}
	return d, nil
}

func serviceNameForInspect(inspect container.ContainerInspectModel, opt ContainerToBottleOptions, single bool) string {
	if single {
		serviceName := strings.TrimSpace(opt.ServiceName)
		if serviceName == "" {
			serviceName = "app"
		}
		serviceName = sanitizeName(serviceName)
		if serviceName != "" {
			return serviceName
		}
		return "app"
	}
	name := sanitizeName(inspect.Name)
	if name != "" {
		return name
	}
	name = sanitizeName(inspect.ContainerId)
	if name != "" {
		return name
	}
	return "service"
}

func inferServiceDependencies(services []ServiceDraft) {
	serviceNames := make([]string, 0, len(services))
	for _, svc := range services {
		serviceNames = append(serviceNames, svc.Name)
	}
	sort.Strings(serviceNames)

	for i := range services {
		deps := map[string]struct{}{}
		for _, env := range services[i].Env {
			for _, name := range serviceNames {
				if name == services[i].Name {
					continue
				}
				if envReferencesService(env.Value, name) {
					deps[name] = struct{}{}
				}
			}
		}
		if len(deps) == 0 {
			continue
		}
		services[i].DependsOn = make([]string, 0, len(deps))
		for dep := range deps {
			services[i].DependsOn = append(services[i].DependsOn, dep)
		}
		sort.Strings(services[i].DependsOn)
	}
}

func envReferencesService(value, serviceName string) bool {
	value = strings.TrimSpace(value)
	serviceName = strings.TrimSpace(serviceName)
	if value == "" || serviceName == "" {
		return false
	}
	if value == serviceName {
		return true
	}
	for _, token := range splitServiceReferenceTokens(value) {
		if token == serviceName {
			return true
		}
	}
	return false
}

func splitServiceReferenceTokens(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-')
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

func displayContainerRef(inspect container.ContainerInspectModel) string {
	if inspect.Name != "" {
		return "container/" + inspect.Name
	}
	if inspect.ContainerId != "" {
		return "container/" + inspect.ContainerId
	}
	return "container"
}

func formatImage(repo, ref string) string {
	repo = strings.TrimSpace(repo)
	ref = strings.TrimSpace(ref)
	if repo == "" {
		return ref
	}
	if ref == "" {
		return repo
	}
	if strings.Contains(repo, ":") || strings.Contains(repo, "@") {
		return repo
	}
	if strings.HasPrefix(ref, "sha256:") {
		return repo + "@" + ref
	}
	return repo + ":" + ref
}

func commandFromInspect(inspect container.ContainerInspectModel) []string {
	if len(inspect.Command) > 0 {
		return cleanStrings(inspect.Command)
	}
	process, _ := inspect.Config["process"].(map[string]any)
	if args, ok := stringSliceFromAny(process["args"]); ok {
		return cleanStrings(args)
	}
	return nil
}

func envFromInspect(inspect container.ContainerInspectModel, includeImageEnv bool) []EnvVar {
	process, _ := inspect.Config["process"].(map[string]any)
	envStrings, ok := stringSliceFromAny(process["env"])
	if !ok || len(envStrings) == 0 {
		return nil
	}
	vars := make([]EnvVar, 0, len(envStrings))
	seen := map[string]struct{}{}
	for _, item := range envStrings {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if !includeImageEnv && isDefaultImageEnv(key) {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		vars = append(vars, EnvVar{Key: key, Value: value, Sensitive: IsSecretLikeKey(key)})
	}
	sort.SliceStable(vars, func(i, j int) bool { return vars[i].Key < vars[j].Key })
	return vars
}

func portsFromInspect(inspect container.ContainerInspectModel) []PortMapping {
	if len(inspect.Forwards) == 0 {
		return nil
	}
	ports := make([]PortMapping, 0, len(inspect.Forwards))
	seen := map[string]struct{}{}
	for _, f := range inspect.Forwards {
		if f.HostPort <= 0 || f.ContainerPort <= 0 {
			continue
		}
		protocol := strings.ToLower(strings.TrimSpace(f.Protocol))
		if protocol == "" {
			protocol = "tcp"
		}
		key := fmt.Sprintf("%d:%d:%s", f.HostPort, f.ContainerPort, protocol)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		ports = append(ports, PortMapping{HostPort: f.HostPort, ContainerPort: f.ContainerPort, Protocol: protocol})
	}
	sort.SliceStable(ports, func(i, j int) bool {
		if ports[i].HostPort != ports[j].HostPort {
			return ports[i].HostPort < ports[j].HostPort
		}
		if ports[i].ContainerPort != ports[j].ContainerPort {
			return ports[i].ContainerPort < ports[j].ContainerPort
		}
		return ports[i].Protocol < ports[j].Protocol
	})
	return ports
}

func isBindMount(mountType string, options []string) bool {
	mountType = strings.ToLower(strings.TrimSpace(mountType))
	if mountType == "bind" {
		return true
	}
	if mountType != "" {
		return false
	}
	return hasMountOption(options, "bind") || hasMountOption(options, "rbind")
}

func mountsFromInspect(inspect container.ContainerInspectModel) []MountMapping {
	items, ok := inspect.Config["mounts"].([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	mounts := make([]MountMapping, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		mountType, _ := m["type"].(string)
		options, _ := stringSliceFromAny(m["options"])
		if !isBindMount(mountType, options) {
			continue
		}
		source, _ := m["source"].(string)
		dest, _ := m["destination"].(string)
		source = strings.TrimSpace(source)
		dest = strings.TrimSpace(dest)
		if source == "" || dest == "" || shouldSkipMount(source, dest) {
			continue
		}
		readOnly := hasMountOption(options, "ro")
		key := source + "\x00" + dest
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		mounts = append(mounts, MountMapping{Source: source, Destination: dest, ReadOnly: readOnly, Options: options})
	}
	sort.SliceStable(mounts, func(i, j int) bool {
		if mounts[i].Destination != mounts[j].Destination {
			return mounts[i].Destination < mounts[j].Destination
		}
		return mounts[i].Source < mounts[j].Source
	})
	return mounts
}

func stringSliceFromAny(value any) ([]string, bool) {
	switch v := value.(type) {
	case []string:
		return v, true
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, false
			}
			out = append(out, s)
		}
		return out, true
	default:
		return nil, false
	}
}

func cleanStrings(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func isDefaultImageEnv(key string) bool {
	switch strings.ToUpper(key) {
	case "PATH", "HOSTNAME", "HOME", "TERM":
		return true
	default:
		return false
	}
}

func shouldSkipMount(source, dest string) bool {
	if strings.HasPrefix(source, "/var/lib/raind/") || strings.HasPrefix(source, "/run/raind/") {
		return true
	}
	switch filepath.Clean(dest) {
	case "/proc", "/sys", "/dev", "/etc/hosts", "/etc/resolv.conf", "/etc/hostname":
		return true
	}
	return false
}

func hasMountOption(options []string, needle string) bool {
	for _, opt := range options {
		if strings.EqualFold(strings.TrimSpace(opt), needle) {
			return true
		}
	}
	return false
}

func sanitizeName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(name) {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	return out
}
