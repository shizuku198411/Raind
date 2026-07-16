package resourcecommand

import (
	"fmt"
	"raind/internal/raind/core/pod"
	"strings"

	"github.com/urfave/cli/v2"
)

func CommandApply() *cli.Command {
	return &cli.Command{
		Name:  "apply",
		Usage: "apply resources from yaml",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "file",
				Aliases:  []string{"f"},
				Usage:    "resource definition yaml file",
				Required: true,
			},
		},
		Action: runApply,
	}
}

func runApply(ctx *cli.Context) error {
	yamlPath := ctx.String("file")
	if yamlPath == "" {
		return fmt.Errorf("missing required flag: -f, --file (yaml file)")
	}

	service := pod.NewServicePodApply()
	result, err := service.Apply(
		pod.ServicePodApplyModel{
			FilePath: yamlPath,
		},
	)
	if err != nil {
		return err
	}

	printAppliedResources(result)
	return nil
}

func printAppliedResources(result pod.ApplyResponseDataModel) {
	printApplyWarnings(result.Warnings)
	for _, ns := range result.Namespaces {
		if ns.Network == "" {
			fmt.Printf("namespace: %s %s\n", ns.Name, applyAction(ns.Action))
			continue
		}
		fmt.Printf("namespace: %s %s (network: %s)\n", ns.Name, applyAction(ns.Action), ns.Network)
	}
	for _, cm := range result.ConfigMaps {
		fmt.Printf("configmap: %s %s\n", cm.ConfigMapId, applyAction(cm.Action))
	}
	for _, secret := range result.Secrets {
		fmt.Printf("secret: %s %s\n", secret.SecretId, applyAction(secret.Action))
	}
	for _, np := range result.NetworkPolicies {
		fmt.Printf("networkpolicy: %s %s (generated rules: %d)\n", np.NetworkPolicyId, applyAction(np.Action), np.GeneratedRules)
	}
	for _, pvc := range result.PersistentVolumeClaims {
		fmt.Printf("persistentvolumeclaim: %s %s (requested: %s, reclaim: %s)\n", pvc.PVCId, applyAction(pvc.Action), pvc.RequestedStorage, pvc.ReclaimPolicy)
	}
	for _, p := range result.Pods {
		if p.ReplicaSetId != "" {
			fmt.Printf("replicaset: %s %s\n", p.ReplicaSetId, applyAction(p.Action))
			continue
		}
		fmt.Printf("pod: %s %s\n", p.PodId, applyAction(p.Action))
	}
	for _, rs := range result.ReplicaSets {
		fmt.Printf("replicaset: %s %s\n", rs.ReplicaSetId, applyAction(rs.Action))
	}
	for _, deploy := range result.Deployments {
		fmt.Printf("deployment: %s %s\n", deploy.DeploymentId, applyAction(deploy.Action))
	}
	for _, svc := range result.Services {
		fmt.Printf("service: %s %s\n", svc.ServiceId, applyAction(svc.Action))
	}
	for _, in := range result.Ingresses {
		if len(in.TLSHosts) > 0 {
			fmt.Printf("ingress: %s %s (tls: enabled, hosts: %s)\n", in.IngressId, applyAction(in.Action), strings.Join(in.TLSHosts, ","))
			continue
		}
		fmt.Printf("ingress: %s %s\n", in.IngressId, applyAction(in.Action))
	}
	if len(result.Namespaces) == 0 && len(result.ConfigMaps) == 0 && len(result.Secrets) == 0 && len(result.NetworkPolicies) == 0 && len(result.PersistentVolumeClaims) == 0 && len(result.Pods) == 0 && len(result.ReplicaSets) == 0 && len(result.Deployments) == 0 && len(result.Services) == 0 && len(result.Ingresses) == 0 {
		fmt.Println("resource applied")
	}
}

func printApplyWarnings(warnings []pod.WarningInfo) {
	for _, warning := range warnings {
		fmt.Printf("warning (%s)\n", formatWarning(warning.Kind, warning.Namespace, warning.Name, warning.Field, warning.Message))
	}
}

func applyAction(action string) string {
	if action == "" {
		return "applied"
	}
	return action
}

func formatWarning(kind, namespace, name, field, message string) string {
	var target string
	if kind != "" {
		target = kind
	}
	if namespace != "" || name != "" {
		if target != "" {
			target += " "
		}
		if namespace != "" {
			target += namespace + "/"
		}
		target += name
	}
	if field != "" {
		if target != "" {
			target += " "
		}
		target += field
	}
	if target == "" {
		return message
	}
	return target + ": " + message
}
