package containercommand

import (
	"raind/internal/raind/core/container"

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
				Name:  "device",
				Usage: "add a host device to the container (SRC[:DST[:rwm]])",
			},
			&cli.StringSliceFlag{
				Name:    "env",
				Aliases: []string{"e"},
				Usage:   "environment variables",
			},
			&cli.StringSliceFlag{
				Name:  "cap-add",
				Usage: "add Linux capabilities (e.g. CAP_NET_ADMIN)",
			},
			&cli.StringSliceFlag{
				Name:  "cap-drop",
				Usage: "drop Linux capabilities (e.g. CAP_NET_RAW)",
			},
			&cli.StringFlag{
				Name:  "security-profile",
				Usage: "security profile to apply (e.g. default, dev, deploy)",
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
			&cli.BoolFlag{
				Name:  "rootless",
				Usage: "run the container with a user namespace and non-root host ID mapping",
				Value: false,
			},
			&cli.StringFlag{
				Name:  "rootless-mode",
				Usage: "rootless ID mapping mode [shifted-root|login-root]",
				Value: "shifted-root",
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
	opt_device, err := validateDeviceFlag(ctx.StringSlice("device"))
	if err != nil {
		return err
	}
	opt_env := ctx.StringSlice("env")
	opt_capAdd := ctx.StringSlice("cap-add")
	opt_capDrop := ctx.StringSlice("cap-drop")
	opt_securityProfile := ctx.String("security-profile")
	opt_tty := ctx.Bool("tty")
	opt_rm := ctx.Bool("rm")
	opt_name := ctx.String("name")
	opt_rootless, opt_rootlessMode, opt_rootlessRootUID, opt_rootlessRootGID, err := rootlessOptionsFromCLI(ctx)
	if err != nil {
		return err
	}

	service := container.NewServiceContainerRun()
	if err := service.Run(
		container.ServiceRunModel{
			Image:           image,
			Command:         command,
			Network:         opt_network,
			Volume:          opt_volume,
			Publish:         opt_publish,
			Device:          opt_device,
			Env:             opt_env,
			CapAdd:          opt_capAdd,
			CapDrop:         opt_capDrop,
			SecurityProfile: opt_securityProfile,
			Tty:             opt_tty,
			Rm:              opt_rm,
			Name:            opt_name,
			Rootless:        opt_rootless,
			RootlessMode:    opt_rootlessMode,
			RootlessRootUID: opt_rootlessRootUID,
			RootlessRootGID: opt_rootlessRootGID,
		},
	); err != nil {
		return err
	}

	return nil
}
