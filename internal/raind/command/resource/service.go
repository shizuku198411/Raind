package resourcecommand

import (
	servicecommand "raind/internal/raind/command/service"
	watchcommand "raind/internal/raind/command/watch"
	coreservice "raind/internal/raind/core/service"

	"github.com/urfave/cli/v2"
)

func CommandService() *cli.Command {
	return &cli.Command{
		Name:  "service",
		Usage: "service resource operation",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "filter by namespace"},
			watchcommand.Flag(),
		},
		Action: func(ctx *cli.Context) error {
			return runWaitOrHelp(ctx, func() error {
				return coreservice.NewServiceServiceList().List(ctx.String("namespace"))
			})
		},
		Subcommands: []*cli.Command{
			servicecommand.CommandCreate(),
			servicecommand.CommandList(),
			servicecommand.CommandShow(),
			servicecommand.CommandRemove(),
		},
	}
}
