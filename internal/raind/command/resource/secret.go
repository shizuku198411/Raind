package resourcecommand

import (
	secretcommand "raind/internal/raind/command/secret"
	watchcommand "raind/internal/raind/command/watch"
	"raind/internal/raind/core/secret"

	"github.com/urfave/cli/v2"
)

func CommandSecret() *cli.Command {
	return &cli.Command{
		Name:  "secret",
		Usage: "secret resource operation",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "filter by namespace"},
			watchcommand.Flag(),
		},
		Action: func(ctx *cli.Context) error {
			return runWaitOrHelp(ctx, func() error {
				return secret.NewServiceList().List(ctx.String("namespace"))
			})
		},
		Subcommands: []*cli.Command{
			secretcommand.CommandList(),
			secretcommand.CommandShow(),
			secretcommand.CommandRemove(),
		},
	}
}
