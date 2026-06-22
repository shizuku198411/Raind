package promote

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type ResourceFile struct {
	Name string
	Data []byte
}

type RenderResourcesOptions struct {
	IngressHost           string
	ServiceType           string
	PreserveHostPorts     bool
	PreserveSensitiveData bool
}

func RenderResourceFiles(d BottleDraft, opt RenderResourcesOptions) ([]ResourceFile, error) {
	namespace := sanitizeName(d.BottleName)
	if namespace == "" {
		namespace = "default"
	}
	services := bottleDraftServices(d)
	services = prepareResourceServices(namespace, services, d.Policies)
	sort.SliceStable(services, func(i, j int) bool { return services[i].Name < services[j].Name })

	files := []ResourceFile{{Name: "00-namespace.yaml", Data: renderNamespace(namespace)}}
	if data := renderConfigMaps(namespace, services); len(data) > 0 {
		files = append(files, ResourceFile{Name: "01-configmap.yaml", Data: data})
	}
	if data := renderSecrets(namespace, services, opt); len(data) > 0 {
		name := "02-secret.example.yaml"
		if opt.PreserveSensitiveData {
			name = "02-secret.yaml"
		}
		files = append(files, ResourceFile{Name: name, Data: data})
	}
	if data := renderPVCs(namespace, services); len(data) > 0 {
		files = append(files, ResourceFile{Name: "03-pvcs.yaml", Data: data})
	}
	files = append(files, ResourceFile{Name: "04-deployments.yaml", Data: renderDeployments(namespace, services)})
	if data := renderServices(namespace, services, opt); len(data) > 0 {
		files = append(files, ResourceFile{Name: "05-services.yaml", Data: data})
	}
	ingressHost := effectiveIngressHost(d, opt)
	if ingressHost != "" {
		data := renderIngress(namespace, services, ingressHost)
		if len(data) == 0 {
			return nil, fmt.Errorf("--ingress-host was specified, but no TCP service port was available for an Ingress backend")
		}
		files = append(files, ResourceFile{Name: "06-ingress.yaml", Data: data})
	}
	if data := renderNetworkPolicies(namespace, d.Policies); len(data) > 0 {
		files = append(files, ResourceFile{Name: "07-networkpolicies.yaml", Data: data})
	}
	files = append(files, ResourceFile{Name: "REVIEW.md", Data: RenderResourceReview(d, files, RenderResourcesOptions{IngressHost: ingressHost})})
	files = append(files, ResourceFile{Name: "all.yaml", Data: renderAllResources(files)})
	return files, nil
}

func effectiveIngressHost(d BottleDraft, opt RenderResourcesOptions) string {
	if host := strings.TrimSpace(opt.IngressHost); host != "" {
		return host
	}
	return strings.TrimSpace(d.IngressHost)
}

func prepareResourceServices(namespace string, services []ServiceDraft, policies []PolicyDraft) []ServiceDraft {
	serviceNames := map[string]struct{}{}
	out := make([]ServiceDraft, 0, len(services))
	for _, svc := range services {
		copySvc := svc
		copySvc.Command = append([]string{}, svc.Command...)
		copySvc.Env = rewriteServiceHostEnv(namespace, svc.Env, services)
		copySvc.Ports = append([]PortMapping{}, svc.Ports...)
		copySvc.Mounts = append([]MountMapping{}, svc.Mounts...)
		copySvc.CapAdd = append([]string{}, svc.CapAdd...)
		copySvc.CapDrop = append([]string{}, svc.CapDrop...)
		copySvc.DependsOn = append([]string{}, svc.DependsOn...)
		serviceNames[copySvc.Name] = struct{}{}
		out = append(out, copySvc)
	}
	index := map[string]int{}
	for i := range out {
		index[out[i].Name] = i
	}
	for _, p := range policies {
		dst := strings.TrimSpace(p.Destination)
		if dst == "" || p.DestPort <= 0 {
			continue
		}
		if _, ok := serviceNames[dst]; !ok {
			continue
		}
		protocol := strings.ToLower(strings.TrimSpace(p.Protocol))
		if protocol == "" {
			protocol = "tcp"
		}
		if protocol != "tcp" && protocol != "udp" {
			continue
		}
		i := index[dst]
		if hasPort(out[i].Ports, p.DestPort, protocol) {
			continue
		}
		out[i].Ports = append(out[i].Ports, PortMapping{ContainerPort: p.DestPort, Protocol: protocol})
		sort.SliceStable(out[i].Ports, func(a, b int) bool {
			if out[i].Ports[a].ContainerPort != out[i].Ports[b].ContainerPort {
				return out[i].Ports[a].ContainerPort < out[i].Ports[b].ContainerPort
			}
			if out[i].Ports[a].HostPort != out[i].Ports[b].HostPort {
				return out[i].Ports[a].HostPort < out[i].Ports[b].HostPort
			}
			return out[i].Ports[a].Protocol < out[i].Ports[b].Protocol
		})
	}
	return out
}

func rewriteServiceHostEnv(namespace string, envs []EnvVar, services []ServiceDraft) []EnvVar {
	serviceNames := map[string]string{}
	for _, svc := range services {
		if svc.Name == "" {
			continue
		}
		serviceNames[svc.Name] = serviceFQDN(svc.Name, namespace)
	}
	out := make([]EnvVar, 0, len(envs))
	for _, env := range envs {
		copyEnv := env
		if !env.Sensitive {
			copyEnv.Value = rewriteServiceHostValue(env.Value, serviceNames)
		}
		out = append(out, copyEnv)
	}
	return out
}

func rewriteServiceHostValue(value string, serviceNames map[string]string) string {
	trimmed := strings.TrimSpace(value)
	if fqdn, ok := serviceNames[trimmed]; ok {
		return fqdn
	}
	host, port, ok := strings.Cut(trimmed, ":")
	if ok {
		if fqdn, found := serviceNames[host]; found && strings.TrimSpace(port) != "" {
			return fqdn + ":" + port
		}
	}
	return value
}

func serviceFQDN(service string, namespace string) string {
	return sanitizeName(service) + "." + sanitizeName(namespace) + ".svc.cluster.local"
}

func hasPort(ports []PortMapping, port int, protocol string) bool {
	for _, p := range ports {
		if p.ContainerPort != port {
			continue
		}
		if strings.EqualFold(defaultProtocol(p.Protocol), protocol) {
			return true
		}
	}
	return false
}

func renderNamespace(namespace string) []byte {
	var b bytes.Buffer
	fmt.Fprintln(&b, "# Generated by Raind Promote from a Bottlefile.")
	fmt.Fprintln(&b, "# Review generated manifests before applying them.")
	fmt.Fprintln(&b, "apiVersion: v1")
	fmt.Fprintln(&b, "kind: Namespace")
	fmt.Fprintln(&b, "metadata:")
	fmt.Fprintf(&b, "  name: %s\n", quoteYAMLString(namespace))
	return b.Bytes()
}

func renderConfigMaps(namespace string, services []ServiceDraft) []byte {
	var b bytes.Buffer
	for _, svc := range services {
		items := nonSecretEnv(svc.Env)
		if len(items) == 0 {
			continue
		}
		writeDocSeparator(&b)
		fmt.Fprintln(&b, "apiVersion: v1")
		fmt.Fprintln(&b, "kind: ConfigMap")
		fmt.Fprintln(&b, "metadata:")
		fmt.Fprintf(&b, "  name: %s\n", quoteYAMLString(configMapName(svc.Name)))
		fmt.Fprintf(&b, "  namespace: %s\n", quoteYAMLString(namespace))
		fmt.Fprintln(&b, "data:")
		for _, env := range items {
			fmt.Fprintf(&b, "  %s: %s\n", quoteYAMLKey(env.Key), quoteYAMLString(env.Value))
		}
	}
	return b.Bytes()
}

func renderSecrets(namespace string, services []ServiceDraft, opt RenderResourcesOptions) []byte {
	var b bytes.Buffer
	for _, svc := range services {
		items := secretEnv(svc.Env)
		if len(items) == 0 {
			continue
		}
		writeDocSeparator(&b)
		if opt.PreserveSensitiveData {
			fmt.Fprintln(&b, "# Generated for promote strategy runtime validation. Do not share this file.")
		} else {
			fmt.Fprintln(&b, "# Example secret only. Replace placeholders before applying.")
		}
		fmt.Fprintln(&b, "apiVersion: v1")
		fmt.Fprintln(&b, "kind: Secret")
		fmt.Fprintln(&b, "metadata:")
		fmt.Fprintf(&b, "  name: %s\n", quoteYAMLString(secretName(svc.Name)))
		fmt.Fprintf(&b, "  namespace: %s\n", quoteYAMLString(namespace))
		fmt.Fprintln(&b, "type: Opaque")
		fmt.Fprintln(&b, "stringData:")
		for _, env := range items {
			value := "<replace-me>"
			if opt.PreserveSensitiveData {
				value = env.Value
			}
			fmt.Fprintf(&b, "  %s: %s\n", quoteYAMLKey(env.Key), quoteYAMLString(value))
		}
	}
	return b.Bytes()
}

func renderPVCs(namespace string, services []ServiceDraft) []byte {
	var b bytes.Buffer
	seen := map[string]struct{}{}
	for _, svc := range services {
		for i, mount := range svc.Mounts {
			name := pvcName(svc.Name, mount, i)
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			writeDocSeparator(&b)
			fmt.Fprintln(&b, "apiVersion: v1")
			fmt.Fprintln(&b, "kind: PersistentVolumeClaim")
			fmt.Fprintln(&b, "metadata:")
			fmt.Fprintf(&b, "  name: %s\n", quoteYAMLString(name))
			fmt.Fprintf(&b, "  namespace: %s\n", quoteYAMLString(namespace))
			fmt.Fprintln(&b, "  annotations:")
			fmt.Fprintf(&b, "    %s: %s\n", quoteYAMLString("raind.dev/reclaimPolicy"), quoteYAMLString("Retain"))
			fmt.Fprintln(&b, "spec:")
			fmt.Fprintln(&b, "  accessModes:")
			fmt.Fprintln(&b, "    - ReadWriteOnce")
			fmt.Fprintln(&b, "  resources:")
			fmt.Fprintln(&b, "    requests:")
			fmt.Fprintln(&b, "      storage: 1Gi")
		}
	}
	return b.Bytes()
}

func renderDeployments(namespace string, services []ServiceDraft) []byte {
	var b bytes.Buffer
	for _, svc := range services {
		writeDocSeparator(&b)
		fmt.Fprintln(&b, "apiVersion: apps/v1")
		fmt.Fprintln(&b, "kind: Deployment")
		fmt.Fprintln(&b, "metadata:")
		fmt.Fprintf(&b, "  name: %s\n", quoteYAMLString(svc.Name))
		fmt.Fprintf(&b, "  namespace: %s\n", quoteYAMLString(namespace))
		fmt.Fprintln(&b, "spec:")
		fmt.Fprintln(&b, "  replicas: 1")
		fmt.Fprintln(&b, "  selector:")
		fmt.Fprintln(&b, "    matchLabels:")
		fmt.Fprintf(&b, "      app: %s\n", quoteYAMLString(svc.Name))
		fmt.Fprintln(&b, "  template:")
		fmt.Fprintln(&b, "    metadata:")
		fmt.Fprintln(&b, "      labels:")
		fmt.Fprintf(&b, "        app: %s\n", quoteYAMLString(svc.Name))
		fmt.Fprintln(&b, "    spec:")
		if len(svc.Mounts) > 0 {
			fmt.Fprintln(&b, "      volumes:")
			for i, mount := range svc.Mounts {
				vol := volumeName(svc.Name, mount, i)
				fmt.Fprintf(&b, "        - name: %s\n", quoteYAMLString(vol))
				fmt.Fprintln(&b, "          persistentVolumeClaim:")
				fmt.Fprintf(&b, "            claimName: %s\n", quoteYAMLString(pvcName(svc.Name, mount, i)))
			}
		}
		fmt.Fprintln(&b, "      containers:")
		fmt.Fprintf(&b, "        - name: %s\n", quoteYAMLString(svc.Name))
		fmt.Fprintf(&b, "          image: %s\n", quoteYAMLString(svc.Image))
		if len(svc.Command) > 0 {
			fmt.Fprintln(&b, "          command:")
			for _, arg := range svc.Command {
				fmt.Fprintf(&b, "            - %s\n", quoteYAMLString(arg))
			}
		}
		if len(nonSecretEnv(svc.Env)) > 0 || len(secretEnv(svc.Env)) > 0 {
			fmt.Fprintln(&b, "          envFrom:")
			if len(nonSecretEnv(svc.Env)) > 0 {
				fmt.Fprintln(&b, "            - configMapRef:")
				fmt.Fprintf(&b, "                name: %s\n", quoteYAMLString(configMapName(svc.Name)))
			}
			if len(secretEnv(svc.Env)) > 0 {
				fmt.Fprintln(&b, "            - secretRef:")
				fmt.Fprintf(&b, "                name: %s\n", quoteYAMLString(secretName(svc.Name)))
			}
		}
		if len(svc.Ports) > 0 {
			fmt.Fprintln(&b, "          ports:")
			for _, p := range svc.Ports {
				fmt.Fprintf(&b, "            - containerPort: %d\n", p.ContainerPort)
			}
		}
		if len(svc.CapAdd) > 0 || len(svc.CapDrop) > 0 {
			fmt.Fprintln(&b, "          securityContext:")
			fmt.Fprintln(&b, "            capabilities:")
			if len(svc.CapAdd) > 0 {
				fmt.Fprintln(&b, "              add:")
				for _, cap := range svc.CapAdd {
					fmt.Fprintf(&b, "                - %s\n", quoteYAMLString(cap))
				}
			}
			if len(svc.CapDrop) > 0 {
				fmt.Fprintln(&b, "              drop:")
				for _, cap := range svc.CapDrop {
					fmt.Fprintf(&b, "                - %s\n", quoteYAMLString(cap))
				}
			}
		}
		if len(svc.Mounts) > 0 {
			fmt.Fprintln(&b, "          volumeMounts:")
			for i, mount := range svc.Mounts {
				fmt.Fprintf(&b, "            - name: %s\n", quoteYAMLString(volumeName(svc.Name, mount, i)))
				fmt.Fprintf(&b, "              mountPath: %s\n", quoteYAMLString(mount.Destination))
				if mount.ReadOnly {
					fmt.Fprintln(&b, "              readOnly: true")
				}
			}
		}
		if svc.Tty {
			fmt.Fprintln(&b, "          tty: true")
		}
	}
	return b.Bytes()
}

func renderServices(namespace string, services []ServiceDraft, opt RenderResourcesOptions) []byte {
	var b bytes.Buffer
	for _, svc := range services {
		if len(svc.Ports) == 0 {
			continue
		}
		writeDocSeparator(&b)
		fmt.Fprintln(&b, "apiVersion: v1")
		fmt.Fprintln(&b, "kind: Service")
		fmt.Fprintln(&b, "metadata:")
		fmt.Fprintf(&b, "  name: %s\n", quoteYAMLString(svc.Name))
		fmt.Fprintf(&b, "  namespace: %s\n", quoteYAMLString(namespace))
		fmt.Fprintln(&b, "spec:")
		serviceType := resourceServiceType(svc, opt)
		fmt.Fprintf(&b, "  type: %s\n", serviceType)
		fmt.Fprintln(&b, "  selector:")
		fmt.Fprintf(&b, "    app: %s\n", quoteYAMLString(svc.Name))
		fmt.Fprintln(&b, "  ports:")
		for _, p := range svc.Ports {
			protocol := strings.ToUpper(strings.TrimSpace(p.Protocol))
			if protocol == "" {
				protocol = "TCP"
			}
			servicePort := p.ContainerPort
			if opt.PreserveHostPorts && p.HostPort > 0 {
				servicePort = p.HostPort
			}
			fmt.Fprintf(&b, "    - port: %d\n", servicePort)
			fmt.Fprintf(&b, "      targetPort: %d\n", p.ContainerPort)
			fmt.Fprintf(&b, "      protocol: %s\n", quoteYAMLString(protocol))
		}
	}
	return b.Bytes()
}

func resourceServiceType(svc ServiceDraft, opt RenderResourcesOptions) string {
	serviceType := strings.TrimSpace(opt.ServiceType)
	if serviceType == "" {
		return "ClusterIP"
	}
	if opt.PreserveHostPorts && !serviceHasHostPort(svc) {
		return "ClusterIP"
	}
	return serviceType
}

func serviceHasHostPort(svc ServiceDraft) bool {
	for _, port := range svc.Ports {
		if port.HostPort > 0 {
			return true
		}
	}
	return false
}

func renderIngress(namespace string, services []ServiceDraft, host string) []byte {
	var target *ServiceDraft
	var port int
	for i := range services {
		for _, p := range services[i].Ports {
			if (p.Protocol == "" || strings.EqualFold(p.Protocol, "tcp")) && p.HostPort > 0 {
				target = &services[i]
				port = p.ContainerPort
				break
			}
		}
		if target != nil {
			break
		}
	}
	if target == nil {
		for i := range services {
			for _, p := range services[i].Ports {
				if p.Protocol == "" || strings.EqualFold(p.Protocol, "tcp") {
					target = &services[i]
					port = p.ContainerPort
					break
				}
			}
			if target != nil {
				break
			}
		}
	}
	if target == nil || port == 0 {
		return nil
	}
	var b bytes.Buffer
	fmt.Fprintln(&b, "apiVersion: networking.k8s.io/v1")
	fmt.Fprintln(&b, "kind: Ingress")
	fmt.Fprintln(&b, "metadata:")
	fmt.Fprintf(&b, "  name: %s\n", quoteYAMLString(target.Name))
	fmt.Fprintf(&b, "  namespace: %s\n", quoteYAMLString(namespace))
	fmt.Fprintln(&b, "spec:")
	fmt.Fprintln(&b, "  rules:")
	fmt.Fprintf(&b, "    - host: %s\n", quoteYAMLString(strings.ToLower(strings.TrimSpace(host))))
	fmt.Fprintln(&b, "      http:")
	fmt.Fprintln(&b, "        paths:")
	fmt.Fprintln(&b, "          - path: /")
	fmt.Fprintln(&b, "            pathType: Prefix")
	fmt.Fprintln(&b, "            backend:")
	fmt.Fprintln(&b, "              service:")
	fmt.Fprintf(&b, "                name: %s\n", quoteYAMLString(target.Name))
	fmt.Fprintln(&b, "                port:")
	fmt.Fprintf(&b, "                  number: %d\n", port)
	return b.Bytes()
}

func renderNetworkPolicies(namespace string, policies []PolicyDraft) []byte {
	var b bytes.Buffer
	for _, p := range policies {
		if p.Source == "" || p.Destination == "" {
			continue
		}
		protocol := strings.ToUpper(strings.TrimSpace(p.Protocol))
		if protocol == "" {
			protocol = "TCP"
		}
		if protocol != "TCP" && protocol != "UDP" {
			continue
		}
		writeDocSeparator(&b)
		fmt.Fprintln(&b, "apiVersion: networking.k8s.io/v1")
		fmt.Fprintln(&b, "kind: NetworkPolicy")
		fmt.Fprintln(&b, "metadata:")
		fmt.Fprintf(&b, "  name: %s\n", quoteYAMLString(sanitizeName("allow-"+p.Source+"-to-"+p.Destination)))
		fmt.Fprintf(&b, "  namespace: %s\n", quoteYAMLString(namespace))
		fmt.Fprintln(&b, "spec:")
		fmt.Fprintln(&b, "  podSelector:")
		fmt.Fprintln(&b, "    matchLabels:")
		fmt.Fprintf(&b, "      app: %s\n", quoteYAMLString(p.Source))
		fmt.Fprintln(&b, "  egress:")
		fmt.Fprintln(&b, "    - to:")
		fmt.Fprintln(&b, "        - podSelector:")
		fmt.Fprintln(&b, "            matchLabels:")
		fmt.Fprintf(&b, "              app: %s\n", quoteYAMLString(p.Destination))
		if p.DestPort > 0 {
			fmt.Fprintln(&b, "      ports:")
			fmt.Fprintf(&b, "        - protocol: %s\n", quoteYAMLString(protocol))
			fmt.Fprintf(&b, "          port: %d\n", p.DestPort)
		}
	}
	return b.Bytes()
}

func RenderResourceReview(d BottleDraft, files []ResourceFile, opt RenderResourcesOptions) []byte {
	var b bytes.Buffer
	fmt.Fprintln(&b, "# Generated Resource Draft Review")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "- Source: %s\n", d.SourceContainer)
	fmt.Fprintf(&b, "- Namespace: `%s`\n", sanitizeName(d.BottleName))
	fmt.Fprintf(&b, "- Services: %d\n", len(bottleDraftServices(d)))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Generated files")
	for _, f := range files {
		if f.Name == "REVIEW.md" || f.Name == "all.yaml" {
			continue
		}
		fmt.Fprintf(&b, "- `%s`\n", f.Name)
	}
	fmt.Fprintln(&b, "- `all.yaml` combines generated resource manifests for current file-based apply flows.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Generated resources")
	for _, svc := range bottleDraftServices(d) {
		fmt.Fprintf(&b, "- Deployment/%s with replicas=1 and image `%s`.\n", svc.Name, svc.Image)
		if len(svc.Ports) > 0 {
			serviceType := resourceServiceType(svc, opt)
			fmt.Fprintf(&b, "- Service/%s as %s using", svc.Name, serviceType)
			if opt.PreserveHostPorts {
				fmt.Fprint(&b, " host-published ports when available")
			} else {
				fmt.Fprint(&b, " container ports")
			}
			for _, p := range svc.Ports {
				servicePort := p.ContainerPort
				if opt.PreserveHostPorts && p.HostPort > 0 {
					servicePort = p.HostPort
				}
				fmt.Fprintf(&b, " %d->%d/%s", servicePort, p.ContainerPort, strings.ToUpper(defaultProtocol(p.Protocol)))
				if !opt.PreserveHostPorts && p.HostPort != 0 && p.HostPort != p.ContainerPort {
					fmt.Fprintf(&b, " (host port %d was not preserved as a Service port)", p.HostPort)
				}
			}
			fmt.Fprintln(&b, ".")
		}
		if len(nonSecretEnv(svc.Env)) > 0 {
			fmt.Fprintf(&b, "- ConfigMap/%s from non-secret env keys.\n", configMapName(svc.Name))
		}
		if len(secretEnv(svc.Env)) > 0 {
			fmt.Fprintf(&b, "- Secret/%s example with placeholder values only.\n", secretName(svc.Name))
		}
		for i, mount := range svc.Mounts {
			fmt.Fprintf(&b, "- PVC/%s for mount `%s` from Bottle source `%s`.\n", pvcName(svc.Name, mount, i), mount.Destination, mount.Source)
		}
		if len(svc.DependsOn) > 0 {
			fmt.Fprintf(&b, "- Service `%s` had depends_on: %s. This is noted but not converted to startup ordering.\n", svc.Name, strings.Join(svc.DependsOn, ", "))
		}
	}
	if strings.TrimSpace(opt.IngressHost) != "" {
		fmt.Fprintf(&b, "- Ingress draft was requested for host `%s`.\n", strings.TrimSpace(opt.IngressHost))
	}
	if len(d.Policies) > 0 {
		fmt.Fprintln(&b, "- NetworkPolicy drafts were generated from Bottle policies when they matched Raind's current podSelector-based subset.")
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Needs review")
	fmt.Fprintln(&b, "- Generated manifests are reviewable drafts, not production-ready Kubernetes configuration.")
	fmt.Fprintln(&b, "- Replace placeholder secret values before applying `02-secret.example.yaml` or omit that file until secrets are ready.")
	fmt.Fprintln(&b, "- Review resource requests, limits, probes, rollout strategy, service account, RBAC, and securityContext.")
	fmt.Fprintln(&b, "- Review PVC names, sizes, storage classes, and whether each original host mount should become persistent storage.")
	if strings.EqualFold(strings.TrimSpace(opt.ServiceType), "NodePort") {
		fmt.Fprintln(&b, "- Review NodePort Services before using these manifests outside Promote Strategy validation.")
	} else {
		fmt.Fprintln(&b, "- Review ClusterIP Services because Bottle host-published ports are not preserved as external exposure.")
	}
	fmt.Fprintln(&b, "- Review NetworkPolicy boundaries. Policies without destination ports allow broader egress within the selected destination pods.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Apply order")
	fmt.Fprintln(&b, "```sh")
	for _, f := range files {
		if f.Name == "REVIEW.md" || f.Name == "all.yaml" {
			continue
		}
		fmt.Fprintf(&b, "raind resource apply -f %s\n", filepath.ToSlash(filepath.Join("manifests", f.Name)))
	}
	fmt.Fprintln(&b, "# Or apply the combined manifest:")
	fmt.Fprintln(&b, "raind resource apply -f manifests/all.yaml")
	fmt.Fprintln(&b, "```")
	return b.Bytes()
}

func renderAllResources(files []ResourceFile) []byte {
	var b bytes.Buffer
	for _, f := range files {
		if f.Name == "REVIEW.md" || f.Name == "all.yaml" || len(f.Data) == 0 {
			continue
		}
		if b.Len() > 0 {
			fmt.Fprintln(&b, "---")
		}
		b.Write(f.Data)
		if !bytes.HasSuffix(f.Data, []byte("\n")) {
			fmt.Fprintln(&b)
		}
	}
	return b.Bytes()
}

func writeDocSeparator(b *bytes.Buffer) {
	if b.Len() > 0 {
		fmt.Fprintln(b, "---")
	}
}

func nonSecretEnv(values []EnvVar) []EnvVar {
	var out []EnvVar
	for _, env := range values {
		if !env.Sensitive {
			out = append(out, env)
		}
	}
	return out
}

func secretEnv(values []EnvVar) []EnvVar {
	var out []EnvVar
	for _, env := range values {
		if env.Sensitive {
			out = append(out, env)
		}
	}
	return out
}

func configMapName(service string) string { return sanitizeName(service + "-config") }
func secretName(service string) string    { return sanitizeName(service + "-secret") }

func pvcName(service string, mount MountMapping, index int) string {
	name := sanitizeName(service + "-" + pathBaseName(mount.Destination))
	if name == "" || name == service {
		name = fmt.Sprintf("%s-data-%d", service, index+1)
	}
	return name
}

func volumeName(service string, mount MountMapping, index int) string {
	return pvcName(service, mount, index)
}

func pathBaseName(path string) string {
	path = strings.Trim(strings.TrimSpace(path), "/")
	if path == "" {
		return "data"
	}
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func quoteYAMLKey(s string) string {
	if s == "" {
		return quoteYAMLString(s)
	}
	for _, r := range s {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.') {
			return quoteYAMLString(s)
		}
	}
	return s
}

func defaultProtocol(protocol string) string {
	protocol = strings.TrimSpace(protocol)
	if protocol == "" {
		return "tcp"
	}
	return strings.ToLower(protocol)
}

func mergeUniqueStrings(a []string, b []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(a)+len(b))
	for _, s := range append(append([]string{}, a...), b...) {
		v := strings.TrimSpace(s)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
