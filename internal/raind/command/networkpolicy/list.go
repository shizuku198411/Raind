package networkpolicycommand

import (
	"raind/internal/raind/core/networkpolicy"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:    "ls",
		Aliases: []string{"list"},
		Usage:   "list networkpolicy resources",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "filter by namespace"},
		},
		Action: func(ctx *cli.Context) error {
			return networkpolicy.NewServiceList().List(ctx.String("namespace"))
		},
	}
}
