package resourcecommand

import (
	"fmt"
	"raind/internal/raind/core/resource"

	"github.com/urfave/cli/v2"
)

func CommandRemove() *cli.Command {
	return &cli.Command{
		Name:  "rm",
		Usage: "remove resources from yaml",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "file",
				Aliases:  []string{"f"},
				Usage:    "resource definition yaml file",
				Required: true,
			},
		},
		Action: runRemove,
	}
}

func runRemove(ctx *cli.Context) error {
	yamlPath := ctx.String("file")
	if yamlPath == "" {
		return fmt.Errorf("missing required flag: -f, --file (yaml file)")
	}

	svc := resource.NewServiceResourceDelete()
	result, err := svc.Delete(
		resource.ServiceResourceDeleteModel{
			FilePath: yamlPath,
		},
	)
	if err != nil {
		return err
	}

	printDeletedResources(result)
	return nil
}

func printDeletedResources(result resource.DeleteResponseModel) {
	printDeleteWarnings(result.Warnings)
	for _, ns := range result.Namespaces {
		fmt.Printf("namespace: %s removed\n", ns.Name)
	}
	for _, cm := range result.ConfigMaps {
		id := cm.ConfigMapId
		if id == "" {
			id = fmt.Sprintf("%s/%s", cm.Namespace, cm.Name)
		}
		fmt.Printf("configmap: %s removed\n", id)
	}
	for _, secret := range result.Secrets {
		id := secret.SecretId
		if id == "" {
			id = fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)
		}
		fmt.Printf("secret: %s removed\n", id)
	}
	for _, np := range result.NetworkPolicies {
		id := np.NetworkPolicyId
		if id == "" {
			id = fmt.Sprintf("%s/%s", np.Namespace, np.Name)
		}
		fmt.Printf("networkpolicy: %s removed\n", id)
	}
	for _, pvc := range result.PersistentVolumeClaims {
		id := pvc.PVCId
		if id == "" {
			id = fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name)
		}
		fmt.Printf("persistentvolumeclaim: %s removed (reclaim: %s)\n", id, pvc.ReclaimPolicy)
	}
	for _, pod := range result.Pods {
		id := pod.PodId
		if id == "" {
			id = fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
		}
		fmt.Printf("pod: %s removed\n", id)
	}
	for _, rs := range result.ReplicaSets {
		id := rs.ReplicaSetId
		if id == "" {
			id = fmt.Sprintf("%s/%s", rs.Namespace, rs.Name)
		}
		fmt.Printf("replicaset: %s removed\n", id)
	}
	for _, deploy := range result.Deployments {
		id := deploy.DeploymentId
		if id == "" {
			id = fmt.Sprintf("%s/%s", deploy.Namespace, deploy.Name)
		}
		fmt.Printf("deployment: %s removed\n", id)
	}
	for _, svc := range result.Services {
		id := svc.ServiceId
		if id == "" {
			id = fmt.Sprintf("%s/%s", svc.Namespace, svc.Name)
		}
		fmt.Printf("service: %s removed\n", id)
	}
	for _, in := range result.Ingresses {
		id := in.IngressId
		if id == "" {
			id = fmt.Sprintf("%s/%s", in.Namespace, in.Name)
		}
		fmt.Printf("ingress: %s removed\n", id)
	}
	if len(result.Namespaces) == 0 && len(result.ConfigMaps) == 0 && len(result.Secrets) == 0 && len(result.NetworkPolicies) == 0 && len(result.PersistentVolumeClaims) == 0 && len(result.Pods) == 0 && len(result.ReplicaSets) == 0 && len(result.Deployments) == 0 && len(result.Services) == 0 && len(result.Ingresses) == 0 {
		fmt.Println("no resources removed")
	}
}

func printDeleteWarnings(warnings []resource.WarningModel) {
	for _, warning := range warnings {
		fmt.Printf("warning: %s\n", formatWarning(warning.Kind, warning.Namespace, warning.Name, warning.Field, warning.Message))
	}
}
