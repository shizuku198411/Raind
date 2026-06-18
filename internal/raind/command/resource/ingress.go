package resourcecommand

import (
	ingresscommand "raind/internal/raind/command/ingress"
	watchcommand "raind/internal/raind/command/watch"
	"raind/internal/raind/core/ingress"

	"github.com/urfave/cli/v2"
)

func CommandIngress() *cli.Command {
	return &cli.Command{
		Name:    "ingress",
		Aliases: []string{"ing"},
		Usage:   "ingress resource operation",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "filter by namespace"},
			watchcommand.Flag(),
		},
		Action: func(ctx *cli.Context) error {
			return runWaitOrHelp(ctx, func() error {
				return ingress.NewServiceIngressList().List(ctx.String("namespace"))
			})
		},
		Subcommands: []*cli.Command{
			ingresscommand.CommandList(),
		},
	}
}
