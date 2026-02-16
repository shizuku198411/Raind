package podcommand

import (
	"fmt"
	"raind/internal/core/pod"

	"github.com/urfave/cli/v2"
)

func CommandStart() *cli.Command {
	return &cli.Command{
		Name:      "start",
		Usage:     "start a pod",
		ArgsUsage: "<pod-id>",
		Action:    runStart,
	}
}

func runStart(ctx *cli.Context) error {
	podId := ctx.Args().Get(0)
	if podId == "" {
		return fmt.Errorf("pod-id required")
	}

	service := pod.NewServicePodStart()
	if err := service.Start(pod.ServicePodStartModel{Id: podId}); err != nil {
		return err
	}
	return nil
}
