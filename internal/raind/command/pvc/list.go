package pvccommand

import (
	watchcommand "raind/internal/raind/command/watch"
	"raind/internal/raind/core/pvc"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:    "ls",
		Aliases: []string{"list"},
		Usage:   "list persistentvolumeclaim resources",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "filter by namespace"},
			watchcommand.Flag(),
		},
		Action: func(ctx *cli.Context) error {
			return watchcommand.Run(watchcommand.Enabled(ctx), func() error {
				return pvc.NewServiceList().List(ctx.String("namespace"))
			})
		},
	}
}
