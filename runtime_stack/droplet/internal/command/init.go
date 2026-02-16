package command

import (
	"droplet/internal/container"

	"github.com/urfave/cli/v2"
)

func commandInit() *cli.Command {
	return &cli.Command{
		Name:      "init",
		Usage:     "initialize a container",
		ArgsUsage: "<container-id> <fifo-path> <entrypoint>",
		Hidden:    true,
		Action:    runInit,
	}
}

func runInit(ctx *cli.Context) error {
	// retrieve fifo and entrypoint
	containerId := ctx.Args().Get(0)
	fifo := ctx.Args().Get(1)
	args := ctx.Args().Slice()
	entrypoint := args[2:]

	containerInit := container.NewContainerInit()
	err := containerInit.Execute(container.InitOption{
		ContainerId: containerId,
		Fifo:        fifo,
		Entrypoint:  entrypoint,
	})
	if err != nil {
		return err
	}

	return nil
}
