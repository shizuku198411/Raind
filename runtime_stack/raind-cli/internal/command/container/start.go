package containercommand

import (
	"fmt"
	"raind/internal/core/container"

	"github.com/urfave/cli/v2"
)

func CommandStart() *cli.Command {
	return &cli.Command{
		Name:      "start",
		Usage:     "start a container",
		ArgsUsage: "<container-id>",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "tty",
				Aliases: []string{"t"},
				Usage:   "attach tty to container",
				Value:   false,
			},
		},
		Action: runStart,
	}
}

func runStart(ctx *cli.Context) error {
	// container ID
	containerId := ctx.Args().Get(0)
	if containerId == "" {
		return fmt.Errorf("container-id required")
	}
	// option
	opt_tty := ctx.Bool("tty")

	service := container.NewServiceContainerStart()
	if err := service.Start(container.ServiceStartModel{
		Id:  containerId,
		Tty: opt_tty,
	}); err != nil {
		return err
	}
	return nil
}
