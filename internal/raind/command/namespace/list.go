package namespace

import (
	watchcommand "raind/internal/raind/command/watch"
	corenamespace "raind/internal/raind/core/namespace"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:    "ls",
		Aliases: []string{"list"},
		Usage:   "list resource namespaces",
		Flags: []cli.Flag{
			watchcommand.Flag(),
		},
		Action: func(ctx *cli.Context) error {
			return watchcommand.Run(watchcommand.Enabled(ctx), func() error {
				return corenamespace.NewServiceNamespaceList().List()
			})
		},
	}
}
