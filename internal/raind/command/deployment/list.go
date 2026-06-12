package deploymentcommand

import (
	"raind/internal/raind/core/deployment"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:    "ls",
		Usage:   "list deployments",
		Aliases: []string{"get"},
		Action:  runList,
	}
}

func runList(ctx *cli.Context) error {
	service := deployment.NewServiceDeploymentList()
	return service.List()
}
