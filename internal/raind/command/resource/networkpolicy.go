package resourcecommand

import (
	networkpolicycommand "raind/internal/raind/command/networkpolicy"
	watchcommand "raind/internal/raind/command/watch"
	"raind/internal/raind/core/networkpolicy"

	"github.com/urfave/cli/v2"
)

func CommandNetworkPolicy() *cli.Command {
	return &cli.Command{
		Name:    "networkpolicy",
		Aliases: []string{"netpol", "np"},
		Usage:   "networkpolicy resource operation",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "filter by namespace"},
			watchcommand.Flag(),
		},
		Action: func(ctx *cli.Context) error {
			return runWaitOrHelp(ctx, func() error {
				return networkpolicy.NewServiceList().List(ctx.String("namespace"))
			})
		},
		Subcommands: []*cli.Command{
			networkpolicycommand.CommandList(),
			networkpolicycommand.CommandShow(),
			networkpolicycommand.CommandRemove(),
		},
	}
}
