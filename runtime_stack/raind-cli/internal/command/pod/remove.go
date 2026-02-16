package podcommand

import (
	"fmt"
	"raind/internal/core/pod"

	"github.com/urfave/cli/v2"
)

func CommandRemove() *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "remove a pod",
		ArgsUsage: "<pod-id>",
		Action:    runRemove,
	}
}

func runRemove(ctx *cli.Context) error {
	podId := ctx.Args().Get(0)
	if podId == "" {
		return fmt.Errorf("pod-id required")
	}

	service := pod.NewServicePodRemove()
	if err := service.Remove(pod.ServicePodRemoveModel{Id: podId}); err != nil {
		return err
	}
	return nil
}
