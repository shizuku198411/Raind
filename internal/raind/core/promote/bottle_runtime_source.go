package promote

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	bottlecore "raind/internal/raind/core/bottle"
	httpclient "raind/internal/raind/core/client"
	"raind/internal/raind/core/container"
)

func BuildResourceDraftFromRunningBottleFile(path string, opt BottleToResourcesOptions) (BottleDraft, error) {
	body, err := readBottleSourceFile(path)
	if err != nil {
		return BottleDraft{}, err
	}

	spec, err := parseBottleSource(body)
	if err != nil {
		return BottleDraft{}, err
	}
	if strings.TrimSpace(spec.Bottle.Name) == "" {
		return BottleDraft{}, fmt.Errorf("bottle.name is required")
	}

	detail, err := FetchRunningBottleDetail(spec.Bottle.Name)
	if err != nil {
		return BottleDraft{}, err
	}
	return BuildResourceDraftFromBottleDetail(detail, opt)
}

func FetchRunningBottleDetail(target string) (bottlecore.BottleDetailModel, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return bottlecore.BottleDetailModel{}, fmt.Errorf("bottle name is required")
	}

	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return bottlecore.BottleDetailModel{}, err
	}
	if err := httpClient.NewRequest(
		http.MethodGet,
		"/v1/bottle/"+url.PathEscape(target),
		nil,
	); err != nil {
		return bottlecore.BottleDetailModel{}, err
	}

	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return bottlecore.BottleDetailModel{}, fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel bottlecore.DetailResponseModel
	if !httpClient.IsStatusOk(resp) {
		if decodeErr := json.NewDecoder(resp.Body).Decode(&respModel); decodeErr != nil {
			return bottlecore.BottleDetailModel{}, fmt.Errorf("decode response: %w", decodeErr)
		}
		if strings.TrimSpace(respModel.Message) != "" {
			return bottlecore.BottleDetailModel{}, fmt.Errorf("%s", respModel.Message)
		}
		return bottlecore.BottleDetailModel{}, fmt.Errorf("unexpected status: %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return bottlecore.BottleDetailModel{}, fmt.Errorf("decode response: %w", err)
	}

	detail := respModel.Data.Bottle
	if err := requireBottleRunning(detail); err != nil {
		return bottlecore.BottleDetailModel{}, err
	}
	return detail, nil
}

func BuildResourceDraftFromBottleDetail(detail bottlecore.BottleDetailModel, opt BottleToResourcesOptions) (BottleDraft, error) {
	if err := requireBottleRunning(detail); err != nil {
		return BottleDraft{}, err
	}
	if strings.TrimSpace(detail.BottleName) == "" {
		return BottleDraft{}, fmt.Errorf("running bottle name is required")
	}

	serviceNames := sortedDetailServiceNames(detail.Services)
	services := make([]ServiceDraft, 0, len(serviceNames))
	for _, name := range serviceNames {
		svc := detail.Services[name]
		state := detail.Containers[name]
		serviceName := sanitizeName(name)
		if serviceName == "" {
			return BottleDraft{}, fmt.Errorf("service name %q is not valid for generated resources", name)
		}
		services = append(services, ServiceDraft{
			Name:      serviceName,
			Image:     runtimeImage(svc.Image, state.Repository, state.Reference),
			Command:   runtimeCommand(svc.Command, state.Command),
			Env:       envFromBottleStrings(svc.Env),
			Ports:     runtimePorts(svc.Ports, state.Forwards),
			Mounts:    mountsFromBottleStrings(svc.Mount),
			CapAdd:    cleanStrings(svc.CapAdd),
			CapDrop:   cleanStrings(svc.CapDrop),
			Network:   strings.TrimSpace(svc.Network),
			Tty:       svc.Tty,
			DependsOn: cleanStrings(svc.DependsOn),
		})
	}

	policies := make([]PolicyDraft, 0, len(detail.Policies))
	for _, policy := range detail.Policies {
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
		if policies[i].Protocol != policies[j].Protocol {
			return policies[i].Protocol < policies[j].Protocol
		}
		return policies[i].DestPort < policies[j].DestPort
	})

	bottleName := sanitizeName(detail.BottleName)
	if bottleName == "" {
		bottleName = "app"
	}
	namespace := sanitizeName(opt.Namespace)
	if namespace == "" {
		namespace = bottleName
	}

	return BottleDraft{
		SourceContainer: "running bottle/" + detail.BottleName,
		BottleName:      namespace,
		Services:        services,
		Policies:        policies,
		IngressHost:     strings.TrimSpace(opt.IngressHost),
	}, nil
}

func requireBottleRunning(detail bottlecore.BottleDetailModel) error {
	if strings.TrimSpace(detail.BottleName) == "" {
		return fmt.Errorf("bottle detail is empty")
	}
	if len(detail.Services) == 0 {
		return fmt.Errorf("bottle %q has no services", detail.BottleName)
	}
	if len(detail.Containers) == 0 {
		return fmt.Errorf("bottle %q is not running: no service containers were found; run `raind bottle start %s` first", detail.BottleName, detail.BottleName)
	}
	for _, name := range sortedDetailServiceNames(detail.Services) {
		containerState, ok := detail.Containers[name]
		if !ok || strings.TrimSpace(containerState.ContainerId) == "" {
			return fmt.Errorf("bottle %q is not running: service %q has no runtime container; run `raind bottle start %s` first", detail.BottleName, name, detail.BottleName)
		}
		if !strings.EqualFold(strings.TrimSpace(containerState.State), "running") {
			state := strings.TrimSpace(containerState.State)
			if state == "" {
				state = "unknown"
			}
			return fmt.Errorf("bottle %q is not running: service %q container is %s; run `raind bottle start %s` first", detail.BottleName, name, state, detail.BottleName)
		}
	}
	return nil
}

func sortedDetailServiceNames(services map[string]bottlecore.BottleServiceModel) []string {
	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func runtimeImage(specImage string, repository string, reference string) string {
	repository = strings.TrimSpace(repository)
	reference = strings.TrimSpace(reference)
	if repository != "" && reference != "" {
		return formatImage(repository, reference)
	}
	return formatPromoteImageString(specImage)
}

func formatPromoteImageString(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return ""
	}

	if at := strings.Index(image, "@"); at >= 0 {
		return promoteDisplayRepository(image[:at]) + image[at:]
	}

	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon > lastSlash {
		return promoteDisplayRepository(image[:lastColon]) + image[lastColon:]
	}
	return promoteDisplayRepository(image)
}

func runtimeCommand(specCommand []string, runningCommand []string) []string {
	cmd := cleanStrings(runningCommand)
	if len(cmd) > 0 {
		return cmd
	}
	return cleanStrings(specCommand)
}

func runtimePorts(specPorts []string, forwards []container.ForwardInfoModel) []PortMapping {
	if len(forwards) == 0 {
		return portsFromBottleStrings(specPorts)
	}
	out := make([]PortMapping, 0, len(forwards))
	for _, f := range forwards {
		protocol := strings.ToLower(strings.TrimSpace(f.Protocol))
		if protocol == "" {
			protocol = "tcp"
		}
		if f.ContainerPort <= 0 {
			continue
		}
		out = append(out, PortMapping{
			HostPort:      f.HostPort,
			ContainerPort: f.ContainerPort,
			Protocol:      protocol,
		})
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
