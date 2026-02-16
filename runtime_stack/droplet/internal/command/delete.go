package command

import (
	"droplet/internal/container"

	"github.com/urfave/cli/v2"
)

func commandDelete() *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "delete a container",
		ArgsUsage: "<container-id>",
		Action:    runDelete,
	}
}

func runDelete(ctx *cli.Context) error {
	// retrieve container ID
	containerId := ctx.Args().Get(0)

	// delete container
	containerDelete := container.NewContainerDelete()
	err := containerDelete.Delete(container.DeleteOption{
		ContainerId: containerId,
	})
	if err != nil {
		return err
	}

	return nil
}
