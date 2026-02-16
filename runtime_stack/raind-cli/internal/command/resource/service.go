package resourcecommand

import (
	servicecommand "raind/internal/command/service"

	"github.com/urfave/cli/v2"
)

func CommandService() *cli.Command {
	return &cli.Command{
		Name:  "service",
		Usage: "service resource operation",
		Subcommands: []*cli.Command{
			servicecommand.CommandCreate(),
			servicecommand.CommandList(),
			servicecommand.CommandShow(),
			servicecommand.CommandRemove(),
		},
	}
}
