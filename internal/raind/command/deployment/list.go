package deploymentcommand

import (
	watchcommand "raind/internal/raind/command/watch"
	"raind/internal/raind/core/deployment"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:    "ls",
		Usage:   "list deployments",
		Aliases: []string{"get"},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "filter by namespace"},
			watchcommand.Flag(),
		},
		Action: func(ctx *cli.Context) error {
			return watchcommand.Run(watchcommand.Enabled(ctx), func() error {
				return runList(ctx)
			})
		},
	}
}

func runList(ctx *cli.Context) error {
	service := deployment.NewServiceDeploymentList()
	return service.List(ctx.String("namespace"))
}
