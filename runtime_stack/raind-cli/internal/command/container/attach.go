package containercommand

import (
	"fmt"
	"raind/internal/core/container"

	"github.com/urfave/cli/v2"
)

func CommandAttach() *cli.Command {
	return &cli.Command{
		Name:      "attach",
		Usage:     "attach to container",
		ArgsUsage: "<container-id>",
		Action:    runAttach,
	}
}

func runAttach(ctx *cli.Context) error {
	// container ID
	containerId := ctx.Args().Get(0)
	if containerId == "" {
		return fmt.Errorf("container-id required")
	}

	service := container.NewServiceContainerAttach()
	if err := service.Attach(containerId); err != nil {
		return err
	}

	return nil
}
