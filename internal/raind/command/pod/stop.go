package podcommand

import (
	"fmt"
	"raind/internal/raind/core/pod"

	"github.com/urfave/cli/v2"
)

func CommandStop() *cli.Command {
	return &cli.Command{
		Name:      "stop",
		Usage:     "stop a pod",
		ArgsUsage: "<pod-id>",
		Action:    runStop,
	}
}

func runStop(ctx *cli.Context) error {
	podId := ctx.Args().Get(0)
	if podId == "" {
		return fmt.Errorf("pod-id required")
	}

	service := pod.NewServicePodStop()
	if err := service.Stop(pod.ServicePodStopModel{Id: podId}); err != nil {
		return err
	}
	return nil
}
