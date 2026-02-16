package command

import (
	"droplet/internal/container"

	"github.com/urfave/cli/v2"
)

func commandStart() *cli.Command {
	return &cli.Command{
		Name:      "start",
		Usage:     "start a container",
		ArgsUsage: "<container-id>",
		Action:    runStart,
	}
}

func runStart(ctx *cli.Context) error {
	// retrieve container ID
	containerId := ctx.Args().Get(0)

	// start container
	containerStart := container.NewContainerStart()
	err := containerStart.Execute(container.StartOption{
		ContainerId: containerId,
	})
	if err != nil {
		return err
	}

	return nil
}
