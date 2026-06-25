package command

import (
	"raind/internal/droplet/container"

	"github.com/urfave/cli/v2"
)

func commandDelete() *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Usage:     "delete a container",
		ArgsUsage: "<container-id>",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "force",
				Usage: "force delete a running or created container",
			},
		},
		Action: runDelete,
	}
}

func runDelete(ctx *cli.Context) error {
	// retrieve container ID
	containerId := ctx.Args().Get(0)
	if containerId == "" {
		return cli.Exit("container-id is required", 1)
	}

	// delete container
	containerDelete := container.NewContainerDelete()
	err := containerDelete.Delete(container.DeleteOption{
		ContainerId: containerId,
		Force:       ctx.Bool("force"),
	})
	if err != nil {
		return err
	}

	return nil
}
