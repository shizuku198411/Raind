package resourcecommand

import (
	replicasetcommand "raind/internal/raind/command/replicaset"
	watchcommand "raind/internal/raind/command/watch"
	"raind/internal/raind/core/replicaset"

	"github.com/urfave/cli/v2"
)

func CommandReplicaSet() *cli.Command {
	return &cli.Command{
		Name:    "replicaset",
		Usage:   "replicaset resource operation",
		Aliases: []string{"rs"},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "filter by namespace"},
			watchcommand.Flag(),
		},
		Action: func(ctx *cli.Context) error {
			return runWaitOrHelp(ctx, func() error {
				return replicaset.NewServiceReplicaSetList().List(ctx.String("namespace"))
			})
		},
		Subcommands: []*cli.Command{
			replicasetcommand.CommandList(),
			replicasetcommand.CommandShow(),
			replicasetcommand.CommandScale(),
			replicasetcommand.CommandRemove(),
		},
	}
}
