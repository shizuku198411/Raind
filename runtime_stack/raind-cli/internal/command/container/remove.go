package containercommand

import (
	"fmt"
	"raind/internal/core/container"

	"github.com/urfave/cli/v2"
)

func CommandRemove() *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "remove a container",
		ArgsUsage: "<container-id>",
		Action:    runRemove,
	}
}

func runRemove(ctx *cli.Context) error {
	// container id
	containerId := ctx.Args().Get(0)
	if containerId == "" {
		return fmt.Errorf("container-id required")
	}

	service := container.NewServiceContainerRemove()
	if err := service.Remove(container.ServiceRemoveModel{
		Id: containerId,
	}); err != nil {
		return err
	}

	return nil
}
