package containercommand

import (
	"fmt"
	"raind/internal/core/container"
	"strings"

	"github.com/urfave/cli/v2"
)

func CommandCreate() *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "create a container",
		ArgsUsage: "<image:tag> [command(,arg1, arg2, ...)]",
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
				Name:    "interactive",
				Aliases: []string{"i"},
			},
			&cli.StringFlag{
				Name:  "name",
				Usage: "container name",
				Value: "",
			},
			&cli.StringFlag{
				Name:  "pod",
				Usage: "pod id",
				Value: "",
			},
		},
		Action: runCreate,
	}
}

func runCreate(ctx *cli.Context) error {
	// args
	args := ctx.Args().Slice()
	// rtrieve image
	image := ctx.Args().Get(0)
	// retrieve commands
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
	opt_name := ctx.String("name")
	opt_podId := ctx.String("pod")

	service := container.NewServiceContainerCreate()
	containerId, err := service.Create(
		container.ServiceCreateModel{
			Image:   image,
			Command: command,
			Network: opt_network,
			Volume:  opt_volume,
			Publish: opt_publish,
			Env:     opt_env,
			Tty:     opt_tty,
			Name:    opt_name,
			PodId:   opt_podId,
		},
	)
	if err != nil {
		return err
	}

	fmt.Printf("container: %s created\n", containerId)

	return nil
}

func validateVolumeFlag(volumes []string) ([]string, error) {
	for _, s := range volumes {
		parts := strings.Split(s, ":")
		if len(parts) != 2 {
			return []string{}, fmt.Errorf("invalid -v,--volume format: %s, required format: /host/path:/dest/path", s)
		}
	}
	return volumes, nil
}

func validatePublishFlag(publishes []string) ([]string, error) {
	for _, s := range publishes {
		parts := strings.Split(s, ":")
		if len(parts) == 1 || len(parts) >= 4 {
			return []string{}, fmt.Errorf("invalid -p,--publish format: %s, required format: sourceport:hostport[:protocol]", s)
		}
	}
	return publishes, nil
}
