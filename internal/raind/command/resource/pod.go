package resourcecommand

import (
	podcommand "raind/internal/raind/command/pod"
	watchcommand "raind/internal/raind/command/watch"
	"raind/internal/raind/core/pod"

	"github.com/urfave/cli/v2"
)

func CommandPod() *cli.Command {
	return &cli.Command{
		Name:  "pod",
		Usage: "pod resource operation",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "filter by namespace"},
			watchcommand.Flag(),
		},
		Action: func(ctx *cli.Context) error {
			return runWaitOrHelp(ctx, func() error {
				return pod.NewServicePodList().List(ctx.String("namespace"))
			})
		},
		Subcommands: []*cli.Command{
			podcommand.CommandCreate(),
			podcommand.CommandList(),
			podcommand.CommandRemove(),
			podcommand.CommandStart(),
			podcommand.CommandStop(),
		},
	}
}
