package resourcecommand

import (
	namespacecommand "raind/internal/raind/command/namespace"
	watchcommand "raind/internal/raind/command/watch"
	corenamespace "raind/internal/raind/core/namespace"

	"github.com/urfave/cli/v2"
)

func CommandNamespace() *cli.Command {
	return &cli.Command{
		Name:    "namespace",
		Aliases: []string{"ns"},
		Usage:   "namespace resource operation",
		Flags: []cli.Flag{
			watchcommand.Flag(),
		},
		Action: func(ctx *cli.Context) error {
			return runWaitOrHelp(ctx, func() error {
				return corenamespace.NewServiceNamespaceList().List()
			})
		},
		Subcommands: []*cli.Command{
			namespacecommand.CommandCreate(),
			namespacecommand.CommandList(),
			namespacecommand.CommandShow(),
			namespacecommand.CommandRemove(),
		},
	}
}
