package resourcecommand

import (
	replicasetcommand "raind/internal/raind/command/replicaset"

	"github.com/urfave/cli/v2"
)

func CommandReplicaSet() *cli.Command {
	return &cli.Command{
		Name:    "replicaset",
		Usage:   "replicaset resource operation",
		Aliases: []string{"rs"},
		Subcommands: []*cli.Command{
			replicasetcommand.CommandList(),
			replicasetcommand.CommandShow(),
			replicasetcommand.CommandScale(),
			replicasetcommand.CommandRemove(),
		},
	}
}
