package resourcecommand

import (
	ingresscommand "raind/internal/raind/command/ingress"

	"github.com/urfave/cli/v2"
)

func CommandIngress() *cli.Command {
	return &cli.Command{
		Name:    "ingress",
		Aliases: []string{"ing"},
		Usage:   "ingress resource operation",
		Subcommands: []*cli.Command{
			ingresscommand.CommandList(),
		},
	}
}
