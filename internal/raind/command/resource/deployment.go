package resourcecommand

import (
	deploymentcommand "raind/internal/raind/command/deployment"

	"github.com/urfave/cli/v2"
)

func CommandDeployment() *cli.Command {
	return &cli.Command{
		Name:    "deployment",
		Usage:   "deployment resource operation",
		Aliases: []string{"deploy"},
		Subcommands: []*cli.Command{
			deploymentcommand.CommandList(),
			deploymentcommand.CommandShow(),
			deploymentcommand.CommandScale(),
			deploymentcommand.CommandRemove(),
		},
	}
}
