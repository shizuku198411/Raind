package command

import (
	"droplet/internal/container"

	"github.com/urfave/cli/v2"
)

func commandExec() *cli.Command {
	return &cli.Command{
		Name:      "exec",
		Usage:     "exec a container",
		ArgsUsage: "<container-id> <command>",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "tty",
				Usage:   "attach tty to container",
				Aliases: []string{"t"},
			},
		},
		Action: runExec,
	}
}

func runExec(ctx *cli.Context) error {
	// retrieve container id
	containerId := ctx.Args().Get(0)
	// retrieve args
	args := ctx.Args().Slice()
	// options
	// interactive
	tty := ctx.Bool("tty")
	entrypoint := args[1:]

	containerExec := container.NewContainerExec()
	err := containerExec.Exec(container.ExecOption{
		ContainerId: containerId,
		Tty:         tty,
		Entrypoint:  entrypoint,
	})
	if err != nil {
		return err
	}
	return nil
}
