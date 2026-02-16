package resourcecommand

import (
	podcommand "raind/internal/command/pod"

	"github.com/urfave/cli/v2"
)

func CommandPod() *cli.Command {
	return &cli.Command{
		Name:  "pod",
		Usage: "pod resource operation",
		Subcommands: []*cli.Command{
			podcommand.CommandCreate(),
			podcommand.CommandList(),
			podcommand.CommandRemove(),
			podcommand.CommandStart(),
			podcommand.CommandStop(),
		},
	}
}
