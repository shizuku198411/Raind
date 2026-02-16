package command

import (
	"droplet/internal/container"

	"github.com/urfave/cli/v2"
)

func commandShim() *cli.Command {
	return &cli.Command{
		Name:      "shim",
		Usage:     "shim process",
		ArgsUsage: "<container-id> <fifo-path> <entrypoint>",
		Hidden:    true,
		Action:    runShim,
	}
}

func runShim(ctx *cli.Context) error {
	// retrieve fifo and entrypoint
	containerId := ctx.Args().Get(0)
	fifo := ctx.Args().Get(1)
	args := ctx.Args().Slice()
	entrypoint := args[2:]

	containerShim := container.NewContainerShim()
	err := containerShim.Execute(containerId, fifo, entrypoint)
	if err != nil {
		return err
	}

	return nil
}
