package podcommand

import (
	"raind/internal/raind/core/pod"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:  "ls",
		Usage: "list pods",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "filter by namespace"},
		},
		Action: runList,
	}
}

func runList(ctx *cli.Context) error {
	service := pod.NewServicePodList()
	if err := service.List(ctx.String("namespace")); err != nil {
		return err
	}
	return nil
}
