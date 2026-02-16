package podcommand

import (
	"raind/internal/core/pod"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:   "ls",
		Usage:  "list pods",
		Action: runList,
	}
}

func runList(ctx *cli.Context) error {
	service := pod.NewServicePodList()
	if err := service.List(); err != nil {
		return err
	}
	return nil
}
