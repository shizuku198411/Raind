package deploymentcommand

import (
	"fmt"
	"raind/internal/raind/core/deployment"

	"github.com/urfave/cli/v2"
)

func CommandRemove() *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "remove a deployment",
		Aliases:   []string{"delete"},
		ArgsUsage: "<deployment-id>",
		Action:    runRemove,
	}
}

func runRemove(ctx *cli.Context) error {
	id := ctx.Args().Get(0)
	if id == "" {
		return fmt.Errorf("deployment id is required")
	}

	service := deployment.NewServiceDeploymentRemove()
	removedId, err := service.Remove(deployment.ServiceDeploymentRemoveModel{Id: id})
	if err != nil {
		return err
	}

	fmt.Printf("deployment: %s removed\n", removedId)
	return nil
}
