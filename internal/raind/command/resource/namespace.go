package resourcecommand

import (
	namespacecommand "raind/internal/raind/command/namespace"

	"github.com/urfave/cli/v2"
)

func CommandNamespace() *cli.Command {
	return &cli.Command{
		Name:    "namespace",
		Aliases: []string{"ns"},
		Usage:   "namespace resource operation",
		Subcommands: []*cli.Command{
			namespacecommand.CommandCreate(),
			namespacecommand.CommandList(),
			namespacecommand.CommandShow(),
			namespacecommand.CommandRemove(),
		},
	}
}
