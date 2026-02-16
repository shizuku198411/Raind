package command

import (
	"droplet/internal/container"

	"github.com/urfave/cli/v2"
)

func commandAttach() *cli.Command {
	return &cli.Command{
		Name:      "attach",
		Usage:     "attach to container",
		ArgsUsage: "<container-id>",
		Action:    runAttach,
	}
}

func runAttach(ctx *cli.Context) error {
	// retrieve container ID
	containerId := ctx.Args().Get(0)

	// start container
	containerAttach := container.NewContainerAttach()
	err := containerAttach.Execute(container.AttachOption{
		ContainerId: containerId,
	})
	if err != nil {
		return err
	}

	return nil
}
