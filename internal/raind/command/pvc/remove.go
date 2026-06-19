package pvccommand

import (
	"raind/internal/raind/core/pvc"

	"github.com/urfave/cli/v2"
)

func CommandRemove() *cli.Command {
	return &cli.Command{
		Name:    "rm",
		Aliases: []string{"remove", "delete"},
		Usage:   "remove persistentvolumeclaim resource",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "namespace for name lookup"},
		},
		Action: func(ctx *cli.Context) error {
			return pvc.NewServiceRemove().Remove(ctx.Args().First(), ctx.String("namespace"))
		},
	}
}
