package command

import (
	"droplet/internal/container"

	"github.com/urfave/cli/v2"
)

func commandKill() *cli.Command {
	return &cli.Command{
		Name:      "kill",
		Usage:     "kill a container",
		ArgsUsage: "<container-id> [signal]",
		Action:    runKill,
	}
}

func runKill(ctx *cli.Context) error {
	// retrieve container id
	containerId := ctx.Args().Get(0)
	// retrieve signal
	var signal string
	if ctx.NArg() == 2 {
		signal = ctx.Args().Get(1)
	} else {
		signal = "TERM"
	}

	containerKill := container.NewContainerKill()
	err := containerKill.Kill(container.KillOption{
		ContainerId: containerId,
		Signal:      signal,
	})
	if err != nil {
		return err
	}
	return nil
}
