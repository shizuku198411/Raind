package command

import (
	"droplet/internal/container"

	"github.com/urfave/cli/v2"
)

func commandExecShim() *cli.Command {
	return &cli.Command{
		Name:      "exec-shim",
		Usage:     "exec-shim process",
		ArgsUsage: "<container-id> <container-pid> <entrypoint>",
		Hidden:    true,
		Action:    runExecShim,
	}
}

func runExecShim(ctx *cli.Context) error {
	// retrieve fifo and entrypoint
	containerId := ctx.Args().Get(0)
	containerPid := ctx.Args().Get(1)
	args := ctx.Args().Slice()
	entrypoint := args[2:]

	containerShim := container.NewContainerExecShim()
	err := containerShim.Execute(containerId, containerPid, entrypoint)
	if err != nil {
		return err
	}

	return nil
}
