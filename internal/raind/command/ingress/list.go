package ingresscommand

import (
	watchcommand "raind/internal/raind/command/watch"
	"raind/internal/raind/core/ingress"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:  "ls",
		Usage: "list ingresses",
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
	svc := ingress.NewServiceIngressList()
	return svc.List(ctx.String("namespace"))
}
