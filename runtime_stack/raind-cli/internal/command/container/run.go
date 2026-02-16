package containercommand

import (
	"raind/internal/core/container"

	"github.com/urfave/cli/v2"
)

func CommandRun() *cli.Command {
	return &cli.Command{
		Name:      "run",
		Usage:     "run a container (create/start[/attach,optional])",
		ArgsUsage: "<image:tag> [command(,arg1,arg2m ...)]",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "network",
				Usage: "specify container network",
				Value: "raind0",
			},
			&cli.StringSliceFlag{
				Name:    "volume",
				Aliases: []string{"v"},
				Usage:   "bind mount a volume",
			},
			&cli.StringSliceFlag{
				Name:    "publish",
				Aliases: []string{"p"},
				Usage:   "publish a container's port(s) to the host",
			},
			&cli.StringSliceFlag{
				Name:    "env",
				Aliases: []string{"e"},
				Usage:   "environment variables",
			},
			&cli.BoolFlag{
				Name:    "tty",
				Aliases: []string{"t"},
				Usage:   "attach tty to container",
				Value:   false,
			},
			&cli.BoolFlag{
				Name:  "rm",
				Usage: "remove container when process terminated",
				Value: false,
			},
			&cli.StringFlag{
				Name:  "name",
				Usage: "container name",
				Value: "",
			},
		},
		Action: runRun,
	}
}

func runRun(ctx *cli.Context) error {
	// args
	args := ctx.Args().Slice()
	// image
	image := ctx.Args().Get(0)
	// command
	var command []string
	if len(args) >= 2 {
		command = append(command, args[1:]...)
	}
	// option
	opt_network := ctx.String("network")
	opt_volume, err := validateVolumeFlag(ctx.StringSlice("volume"))
	if err != nil {
		return err
	}
	opt_publish, err := validatePublishFlag(ctx.StringSlice("publish"))
	if err != nil {
		return err
	}
	opt_env := ctx.StringSlice("env")
	opt_tty := ctx.Bool("tty")
	opt_rm := ctx.Bool("rm")
	opt_name := ctx.String("name")

	service := container.NewServiceContainerRun()
	if err := service.Run(
		container.ServiceRunModel{
			Image:   image,
			Command: command,
			Network: opt_network,
			Volume:  opt_volume,
			Publish: opt_publish,
			Env:     opt_env,
			Tty:     opt_tty,
			Rm:      opt_rm,
			Name:    opt_name,
		},
	); err != nil {
		return err
	}

	return nil
}
