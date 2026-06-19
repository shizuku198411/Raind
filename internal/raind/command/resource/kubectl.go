package resourcecommand

import (
	"fmt"
	"strconv"
	"strings"

	watchcommand "raind/internal/raind/command/watch"
	configmap "raind/internal/raind/core/configmap"
	deployment "raind/internal/raind/core/deployment"
	ingress "raind/internal/raind/core/ingress"
	namespace "raind/internal/raind/core/namespace"
	networkpolicy "raind/internal/raind/core/networkpolicy"
	pod "raind/internal/raind/core/pod"
	pvc "raind/internal/raind/core/pvc"
	replicaset "raind/internal/raind/core/replicaset"
	resource "raind/internal/raind/core/resource"
	secret "raind/internal/raind/core/secret"
	service "raind/internal/raind/core/service"

	"github.com/urfave/cli/v2"
)

func CommandGet() *cli.Command {
	return &cli.Command{
		Name:      "get",
		Usage:     "list resources",
		ArgsUsage: "<resource>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "filter by namespace"},
			watchcommand.Flag(),
		},
		Action: func(ctx *cli.Context) error {
			return watchcommand.Run(watchcommand.Enabled(ctx), func() error {
				return runKubectlGet(ctx)
			})
		},
	}
}

func CommandDescribe() *cli.Command {
	return &cli.Command{
		Name:      "describe",
		Aliases:   []string{"show"},
		Usage:     "show resource details",
		ArgsUsage: "<resource> <name>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "namespace for name lookup"},
		},
		Action: runKubectlDescribe,
	}
}

func CommandDelete() *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "delete a resource or resources from yaml",
		ArgsUsage: "<resource> <name>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "resource definition yaml file"},
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "namespace for name lookup"},
		},
		Action: runKubectlDelete,
	}
}

func CommandCreate() *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "create a resource",
		ArgsUsage: "<resource> <name>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "network", Usage: "bind namespace to an existing network"},
		},
		Action: runKubectlCreate,
	}
}

func CommandScale() *cli.Command {
	return &cli.Command{
		Name:      "scale",
		Usage:     "scale a scalable resource",
		ArgsUsage: "<resource> <name>",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "replicas", Aliases: []string{"r"}, Usage: "number of replicas"},
		},
		Action: runKubectlScale,
	}
}

func runKubectlGet(ctx *cli.Context) error {
	args := kubectlArgs(ctx)
	kind, _, err := splitKindName(argAt(args, 0), "")
	if err != nil {
		return err
	}
	namespaceFilter := ctx.String("namespace")
	switch kind {
	case resourcePod:
		return pod.NewServicePodList().List(namespaceFilter)
	case resourceReplicaSet:
		return replicaset.NewServiceReplicaSetList().List(namespaceFilter)
	case resourceDeployment:
		return deployment.NewServiceDeploymentList().List(namespaceFilter)
	case resourceService:
		return service.NewServiceServiceList().List(namespaceFilter)
	case resourceIngress:
		return ingress.NewServiceIngressList().List(namespaceFilter)
	case resourceNamespace:
		return namespace.NewServiceNamespaceList().List()
	case resourceConfigMap:
		return configmap.NewServiceList().List(namespaceFilter)
	case resourceSecret:
		return secret.NewServiceList().List(namespaceFilter)
	case resourceNetworkPolicy:
		return networkpolicy.NewServiceList().List(namespaceFilter)
	case resourcePVC:
		return pvc.NewServiceList().List(namespaceFilter)
	default:
		return fmt.Errorf("unsupported resource: %s", ctx.Args().First())
	}
}

func runKubectlDescribe(ctx *cli.Context) error {
	args := kubectlArgs(ctx)
	kind, name, err := splitKindName(argAt(args, 0), argAt(args, 1))
	if err != nil {
		return err
	}
	if err := requireResourceName(kind, name); err != nil {
		return err
	}
	ns := ctx.String("namespace")
	switch kind {
	case resourceReplicaSet:
		return replicaset.NewServiceReplicaSetDetail().Detail(name)
	case resourceDeployment:
		return deployment.NewServiceDeploymentDetail().Detail(name)
	case resourceService:
		return service.NewServiceServiceDetail().Detail(name)
	case resourceNamespace:
		return namespace.NewServiceNamespaceDetail().Detail(name)
	case resourceConfigMap:
		return configmap.NewServiceDetail().Detail(name, ns)
	case resourceSecret:
		return secret.NewServiceDetail().Detail(name, ns)
	case resourceNetworkPolicy:
		return networkpolicy.NewServiceDetail().Detail(name, ns)
	case resourcePVC:
		return pvc.NewServiceDetail().Detail(name, ns)
	case resourcePod, resourceIngress:
		return fmt.Errorf("describe is not supported for %s yet", kind)
	default:
		return fmt.Errorf("unsupported resource: %s", ctx.Args().Get(0))
	}
}

func runKubectlDelete(ctx *cli.Context) error {
	if yamlPath := ctx.String("file"); yamlPath != "" {
		svc := resource.NewServiceResourceDelete()
		result, err := svc.Delete(resource.ServiceResourceDeleteModel{FilePath: yamlPath})
		if err != nil {
			return err
		}
		printDeletedResources(result)
		return nil
	}

	args := kubectlArgs(ctx)
	kind, name, err := splitKindName(argAt(args, 0), argAt(args, 1))
	if err != nil {
		return err
	}
	if err := requireResourceName(kind, name); err != nil {
		return err
	}
	ns := ctx.String("namespace")
	switch kind {
	case resourcePod:
		return pod.NewServicePodRemove().Remove(pod.ServicePodRemoveModel{Id: name})
	case resourceReplicaSet:
		removedId, err := replicaset.NewServiceReplicaSetRemove().Remove(replicaset.ServiceReplicaSetRemoveModel{Id: name})
		if err != nil {
			return err
		}
		fmt.Printf("replicaset: %s removed\n", removedId)
		return nil
	case resourceDeployment:
		removedId, err := deployment.NewServiceDeploymentRemove().Remove(deployment.ServiceDeploymentRemoveModel{Id: name})
		if err != nil {
			return err
		}
		fmt.Printf("deployment: %s removed\n", removedId)
		return nil
	case resourceService:
		removedId, err := service.NewServiceServiceRemove().Remove(service.ServiceServiceRemoveModel{Id: name})
		if err != nil {
			return err
		}
		fmt.Printf("service: %s removed\n", removedId)
		return nil
	case resourceNamespace:
		deleted, err := namespace.NewServiceNamespaceRemove().Remove(name)
		if err != nil {
			return err
		}
		fmt.Printf("namespace: %s removed\n", deleted)
		return nil
	case resourceConfigMap:
		info, err := configmap.NewServiceRemove().Remove(name, ns)
		if err != nil {
			return err
		}
		fmt.Printf("configmap: %s removed\n", info.ConfigMapId)
		return nil
	case resourceSecret:
		info, err := secret.NewServiceRemove().Remove(name, ns)
		if err != nil {
			return err
		}
		fmt.Printf("secret: %s removed\n", info.SecretId)
		return nil
	case resourceNetworkPolicy:
		return networkpolicy.NewServiceRemove().Remove(name, ns)
	case resourcePVC:
		return pvc.NewServiceRemove().Remove(name, ns)
	case resourceIngress:
		return fmt.Errorf("delete is not supported for ingress by id yet; use delete -f")
	default:
		return fmt.Errorf("unsupported resource: %s", ctx.Args().Get(0))
	}
}

func runKubectlCreate(ctx *cli.Context) error {
	args := kubectlArgs(ctx)
	kind, name, err := splitKindName(argAt(args, 0), argAt(args, 1))
	if err != nil {
		return err
	}
	if err := requireResourceName(kind, name); err != nil {
		return err
	}
	switch kind {
	case resourceNamespace:
		info, err := namespace.NewServiceNamespaceCreate().Create(namespace.CreateModel{
			Name:    name,
			Network: ctx.String("network"),
		})
		if err != nil {
			return err
		}
		fmt.Printf("namespace: %s created (network: %s)\n", info.Name, info.Network)
		return nil
	default:
		return fmt.Errorf("create is only supported for namespace; use apply -f for %s", kind)
	}
}

func runKubectlScale(ctx *cli.Context) error {
	args := kubectlArgs(ctx)
	kind, name, err := splitKindName(argAt(args, 0), argAt(args, 1))
	if err != nil {
		return err
	}
	if err := requireResourceName(kind, name); err != nil {
		return err
	}
	replicas, err := kubectlReplicas(ctx)
	if err != nil {
		return err
	}
	if replicas < 0 {
		return fmt.Errorf("replicas must be >= 0")
	}
	switch kind {
	case resourceReplicaSet:
		data, err := replicaset.NewServiceReplicaSetScale().Scale(replicaset.ServiceReplicaSetScaleModel{Id: name, Replicas: replicas})
		if err != nil {
			return err
		}
		id := data.ReplicaSetId
		if id == "" {
			id = name
		}
		fmt.Printf("replicaset: %s scaled to %d\n", id, data.Replicas)
		return nil
	case resourceDeployment:
		data, err := deployment.NewServiceDeploymentScale().Scale(deployment.ServiceDeploymentScaleModel{Id: name, Replicas: replicas})
		if err != nil {
			return err
		}
		id := data.DeploymentId
		if id == "" {
			id = name
		}
		fmt.Printf("deployment: %s scaled to %d\n", id, data.Replicas)
		return nil
	default:
		return fmt.Errorf("scale is only supported for deployment and replicaset")
	}
}

type kubectlResourceKind string

const (
	resourcePod           kubectlResourceKind = "pod"
	resourceReplicaSet    kubectlResourceKind = "replicaset"
	resourceDeployment    kubectlResourceKind = "deployment"
	resourceService       kubectlResourceKind = "service"
	resourceIngress       kubectlResourceKind = "ingress"
	resourceNamespace     kubectlResourceKind = "namespace"
	resourceConfigMap     kubectlResourceKind = "configmap"
	resourceSecret        kubectlResourceKind = "secret"
	resourceNetworkPolicy kubectlResourceKind = "networkpolicy"
	resourcePVC           kubectlResourceKind = "persistentvolumeclaim"
)

func splitKindName(resourceArg, nameArg string) (kubectlResourceKind, string, error) {
	resourceArg = strings.TrimSpace(resourceArg)
	nameArg = strings.TrimSpace(nameArg)
	if resourceArg == "" {
		return "", "", fmt.Errorf("resource is required")
	}
	if strings.Contains(resourceArg, "/") {
		parts := strings.SplitN(resourceArg, "/", 2)
		resourceArg = parts[0]
		if nameArg == "" {
			nameArg = parts[1]
		}
	}
	kind, ok := normalizeResourceKind(resourceArg)
	if !ok {
		return "", "", fmt.Errorf("unsupported resource: %s", resourceArg)
	}
	return kind, nameArg, nil
}

func requireResourceName(kind kubectlResourceKind, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("%s name is required", kind)
	}
	return nil
}

func normalizeResourceKind(raw string) (kubectlResourceKind, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "pod", "pods", "po":
		return resourcePod, true
	case "replicaset", "replicasets", "rs":
		return resourceReplicaSet, true
	case "deployment", "deployments", "deploy":
		return resourceDeployment, true
	case "service", "services", "svc":
		return resourceService, true
	case "ingress", "ingresses", "ing":
		return resourceIngress, true
	case "namespace", "namespaces", "ns":
		return resourceNamespace, true
	case "configmap", "configmaps", "cm":
		return resourceConfigMap, true
	case "secret", "secrets":
		return resourceSecret, true
	case "networkpolicy", "networkpolicies", "netpol", "np":
		return resourceNetworkPolicy, true
	case "persistentvolumeclaim", "persistentvolumeclaims", "pvc":
		return resourcePVC, true
	default:
		return "", false
	}
}

func kubectlReplicas(ctx *cli.Context) (int, error) {
	if ctx.IsSet("replicas") {
		return ctx.Int("replicas"), nil
	}
	args := ctx.Args().Slice()
	for i := 0; i < len(args); i++ {
		if args[i] == "-r" || args[i] == "--replicas" {
			if i+1 >= len(args) {
				return 0, fmt.Errorf("replicas value is required")
			}
			return strconv.Atoi(args[i+1])
		}
		if strings.HasPrefix(args[i], "--replicas=") {
			return strconv.Atoi(strings.TrimPrefix(args[i], "--replicas="))
		}
	}
	return 0, fmt.Errorf("replicas is required")
}

func kubectlArgs(ctx *cli.Context) []string {
	args := ctx.Args().Slice()
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-w" || arg == "--wait" || arg == "--watch":
			continue
		case arg == "-n" || arg == "--namespace":
			i++
			continue
		case strings.HasPrefix(arg, "--namespace="):
			continue
		default:
			out = append(out, arg)
		}
	}
	return out
}

func argAt(args []string, index int) string {
	if index < 0 || index >= len(args) {
		return ""
	}
	return args[index]
}
