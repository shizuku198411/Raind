package resourcecommand

import (
	deploymentcommand "raind/internal/raind/command/deployment"
	watchcommand "raind/internal/raind/command/watch"
	"raind/internal/raind/core/deployment"

	"github.com/urfave/cli/v2"
)

func CommandDeployment() *cli.Command {
	return &cli.Command{
		Name:    "deployment",
		Usage:   "deployment resource operation",
		Aliases: []string{"deploy"},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "filter by namespace"},
			watchcommand.Flag(),
		},
		Action: func(ctx *cli.Context) error {
			return runWaitOrHelp(ctx, func() error {
				return deployment.NewServiceDeploymentList().List(ctx.String("namespace"))
			})
		},
		Subcommands: []*cli.Command{
			deploymentcommand.CommandList(),
			deploymentcommand.CommandShow(),
			deploymentcommand.CommandScale(),
			deploymentcommand.CommandRemove(),
		},
	}
}
