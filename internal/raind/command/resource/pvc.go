package resourcecommand

import (
	pvccommand "raind/internal/raind/command/pvc"
	watchcommand "raind/internal/raind/command/watch"
	"raind/internal/raind/core/pvc"

	"github.com/urfave/cli/v2"
)

func CommandPVC() *cli.Command {
	return &cli.Command{
		Name:    "persistentvolumeclaim",
		Aliases: []string{"pvc"},
		Usage:   "persistentvolumeclaim resource operation",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "filter by namespace"},
			watchcommand.Flag(),
		},
		Action: func(ctx *cli.Context) error {
			return runWaitOrHelp(ctx, func() error {
				return pvc.NewServiceList().List(ctx.String("namespace"))
			})
		},
		Subcommands: []*cli.Command{
			pvccommand.CommandList(),
			pvccommand.CommandShow(),
			pvccommand.CommandRemove(),
		},
	}
}
