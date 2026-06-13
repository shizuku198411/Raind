package namespace

import (
	corenamespace "raind/internal/raind/core/namespace"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:    "ls",
		Aliases: []string{"list"},
		Usage:   "list resource namespaces",
		Action: func(ctx *cli.Context) error {
			return corenamespace.NewServiceNamespaceList().List()
		},
	}
}
