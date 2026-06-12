package deploymentcommand

import (
	"fmt"
	"raind/internal/raind/core/deployment"
	"strconv"

	"github.com/urfave/cli/v2"
)

func CommandScale() *cli.Command {
	return &cli.Command{
		Name:      "scale",
		Usage:     "scale a deployment",
		ArgsUsage: "<deployment-id>",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "replicas",
				Aliases: []string{"r"},
				Usage:   "number of replicas",
			},
		},
		Action: runScale,
	}
}

func runScale(ctx *cli.Context) error {
	id := ctx.Args().Get(0)
	if id == "" {
		return fmt.Errorf("deployment id is required")
	}

	replicas, err := resolveReplicas(ctx)
	if err != nil {
		return err
	}
	if replicas < 0 {
		return fmt.Errorf("replicas must be >= 0")
	}

	service := deployment.NewServiceDeploymentScale()
	data, err := service.Scale(deployment.ServiceDeploymentScaleModel{Id: id, Replicas: replicas})
	if err != nil {
		return err
	}

	targetId := data.DeploymentId
	if targetId == "" {
		targetId = id
	}
	fmt.Printf("deployment: %s scaled to %d\n", targetId, data.Replicas)
	return nil
}

func resolveReplicas(ctx *cli.Context) (int, error) {
	if ctx.IsSet("replicas") {
		return ctx.Int("replicas"), nil
	}
	args := ctx.Args().Slice()
	for i := 1; i < len(args); i++ {
		if args[i] == "-r" || args[i] == "--replicas" {
			if i+1 >= len(args) {
				return 0, fmt.Errorf("replicas value is required")
			}
			return strconv.Atoi(args[i+1])
		}
		if n, err := strconv.Atoi(args[i]); err == nil {
			return n, nil
		}
	}
	return 0, fmt.Errorf("replicas is required")
}
