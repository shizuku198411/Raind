package command

import (
	"raind/internal/droplet/container"

	"github.com/urfave/cli/v2"
)

func commandShim() *cli.Command {
	return &cli.Command{
		Name:      "shim",
		Usage:     "shim process",
		ArgsUsage: "[--tty] <container-id> <fifo-path> <entrypoint>",
		Hidden:    true,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "tty",
				Usage: "enable TTY attach proxy",
			},
		},
		Action: runShim,
	}
}

func runShim(ctx *cli.Context) error {
	// retrieve fifo and entrypoint
	containerId := ctx.Args().Get(0)
	fifo := ctx.Args().Get(1)
	args := ctx.Args().Slice()
	entrypoint := args[2:]

	containerShim := container.NewContainerShim()
	err := containerShim.Execute(container.ShimExecuteOption{
		ContainerId: containerId,
		Fifo:        fifo,
		Entrypoint:  entrypoint,
		Tty:         ctx.Bool("tty"),
	})
	if err != nil {
		return err
	}

	return nil
}
