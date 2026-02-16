package containercommand

import (
	"fmt"
	"raind/internal/core/container"

	"github.com/urfave/cli/v2"
)

func CommandExec() *cli.Command {
	return &cli.Command{
		Name:      "exec",
		Usage:     "exec command inside a container",
		ArgsUsage: "<container-id> [command(,arg1, arg2, ...)]",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "tty",
				Aliases: []string{"t"},
				Usage:   "attach tty to container",
				Value:   false,
			},
		},
		Action: runExec,
	}
}

func runExec(ctx *cli.Context) error {
	// args
	args := ctx.Args().Slice()
	// container id
	containerId := ctx.Args().Get(0)
	// retrieve commands
	var command []string
	if len(args) >= 2 {
		command = append(command, args[1:]...)
	} else {
		return fmt.Errorf("command required")
	}
	opt_tty := ctx.Bool("tty")

	service := container.NewServiceContainerExec()
	if err := service.Exec(
		container.ServiceExecModel{
			ContainerId: containerId,
			Tty:         opt_tty,
			Command:     command,
		},
	); err != nil {
		return err
	}
	return nil
}
