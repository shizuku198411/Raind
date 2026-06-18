package servicecommand

import (
	watchcommand "raind/internal/raind/command/watch"
	"raind/internal/raind/core/service"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:  "ls",
		Usage: "list services",
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
	svc := service.NewServiceServiceList()
	if err := svc.List(ctx.String("namespace")); err != nil {
		return err
	}
	return nil
}
