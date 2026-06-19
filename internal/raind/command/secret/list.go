package secretcommand

import (
	watchcommand "raind/internal/raind/command/watch"
	"raind/internal/raind/core/secret"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:  "ls",
		Usage: "list secrets",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "filter by namespace"},
			watchcommand.Flag(),
		},
		Action: func(ctx *cli.Context) error {
			return watchcommand.Run(watchcommand.Enabled(ctx), func() error {
				return secret.NewServiceList().List(ctx.String("namespace"))
			})
		},
	}
}
