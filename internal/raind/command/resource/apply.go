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
	for _, ns := range result.Namespaces {
		if ns.Network == "" {
			fmt.Printf("namespace: %s applied\n", ns.Name)
			continue
		}
		fmt.Printf("namespace: %s applied (network: %s)\n", ns.Name, ns.Network)
	}
	for _, p := range result.Pods {
		if p.ReplicaSetId != "" {
			fmt.Printf("replicaset: %s applied\n", p.ReplicaSetId)
			continue
		}
		fmt.Printf("pod: %s applied\n", p.PodId)
	}
	for _, rs := range result.ReplicaSets {
		fmt.Printf("replicaset: %s applied\n", rs.ReplicaSetId)
	}
	for _, deploy := range result.Deployments {
		fmt.Printf("deployment: %s applied\n", deploy.DeploymentId)
	}
	for _, svc := range result.Services {
		fmt.Printf("service: %s applied\n", svc.ServiceId)
	}
	for _, in := range result.Ingresses {
		if len(in.TLSHosts) > 0 {
			fmt.Printf("ingress: %s applied (tls: enabled, hosts: %s)\n", in.IngressId, strings.Join(in.TLSHosts, ","))
			continue
		}
		fmt.Printf("ingress: %s applied\n", in.IngressId)
	}
	if len(result.Namespaces) == 0 && len(result.Pods) == 0 && len(result.ReplicaSets) == 0 && len(result.Deployments) == 0 && len(result.Services) == 0 && len(result.Ingresses) == 0 {
		fmt.Println("resource applied")
	}
}
