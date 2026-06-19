package networkpolicycommand

import (
	"raind/internal/raind/core/networkpolicy"

	"github.com/urfave/cli/v2"
)

func CommandShow() *cli.Command {
	return &cli.Command{
		Name:    "show",
		Aliases: []string{"get"},
		Usage:   "show networkpolicy resource",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "namespace for name lookup"},
		},
		Action: func(ctx *cli.Context) error {
			return networkpolicy.NewServiceDetail().Detail(ctx.Args().First(), ctx.String("namespace"))
		},
	}
}
