package containercommand

import (
	"fmt"
	"raind/internal/core/container"

	"github.com/urfave/cli/v2"
)

func CommandStop() *cli.Command {
	return &cli.Command{
		Name:      "stop",
		Usage:     "stop a container",
		ArgsUsage: "<container-id>",
		Action:    runStop,
	}
}

func runStop(ctx *cli.Context) error {
	// container id
	containerId := ctx.Args().Get(0)
	if containerId == "" {
		return fmt.Errorf("container-id required")
	}

	service := container.NewServiceContainerStop()
	if err := service.Stop(container.ServiceStopModel{
		Id: containerId,
	}); err != nil {
		return err
	}
	return nil
}
