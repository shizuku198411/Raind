package resourcecommand

import (
	configmapcommand "raind/internal/raind/command/configmap"
	watchcommand "raind/internal/raind/command/watch"
	"raind/internal/raind/core/configmap"

	"github.com/urfave/cli/v2"
)

func CommandConfigMap() *cli.Command {
	return &cli.Command{
		Name:    "configmap",
		Aliases: []string{"cm"},
		Usage:   "configmap resource operation",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "filter by namespace"},
			watchcommand.Flag(),
		},
		Action: func(ctx *cli.Context) error {
			return runWaitOrHelp(ctx, func() error {
				return configmap.NewServiceList().List(ctx.String("namespace"))
			})
		},
		Subcommands: []*cli.Command{
			configmapcommand.CommandList(),
			configmapcommand.CommandShow(),
			configmapcommand.CommandRemove(),
		},
	}
}
