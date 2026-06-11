package containercommand

import (
	"fmt"
	"raind/internal/raind/core/container"
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
	opt_device, err := validateDeviceFlag(ctx.StringSlice("device"))
	if err != nil {
		return err
	}
	opt_env := ctx.StringSlice("env")
	opt_capAdd := ctx.StringSlice("cap-add")
	opt_capDrop := ctx.StringSlice("cap-drop")
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
			Device:  opt_device,
			Env:     opt_env,
			CapAdd:  opt_capAdd,
			CapDrop: opt_capDrop,
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

func validateDeviceFlag(devices []string) ([]string, error) {
	for _, s := range devices {
		parts := strings.Split(s, ":")
		if len(parts) == 0 || len(parts) > 3 {
			return []string{}, fmt.Errorf("invalid --device format: %s, required format: /src/path[:/dst/path[:rwm]]", s)
		}
		if strings.TrimSpace(parts[0]) == "" {
			return []string{}, fmt.Errorf("invalid --device format: %s, source path is required", s)
		}
		if len(parts) >= 2 && strings.TrimSpace(parts[1]) == "" {
			return []string{}, fmt.Errorf("invalid --device format: %s, destination path is required", s)
		}
		if len(parts) == 3 && strings.TrimSpace(parts[2]) != "" {
			p := strings.ToLower(strings.TrimSpace(parts[2]))
			for _, ch := range p {
				if ch != 'r' && ch != 'w' && ch != 'm' {
					return []string{}, fmt.Errorf("invalid --device permission: %s, expected combination of rwm", s)
				}
			}
		}
	}
	return devices, nil
}
