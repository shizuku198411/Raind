package replicasetcommand

import (
	"raind/internal/raind/core/replicaset"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:    "ls",
		Usage:   "list replicasets",
		Aliases: []string{"get"},
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "filter by namespace"},
		},
		Action: runList,
	}
}

func runList(ctx *cli.Context) error {
	service := replicaset.NewServiceReplicaSetList()
	if err := service.List(ctx.String("namespace")); err != nil {
		return err
	}
	return nil
}
