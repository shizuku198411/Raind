package imagecommand

import (
	"fmt"
	imageservice "raind/internal/core/image"

	"github.com/urfave/cli/v2"
)

func CommandPull() *cli.Command {
	return &cli.Command{
		Name:      "pull",
		Usage:     "download an image from a registry",
		ArgsUsage: "<image:tag>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "os",
				Usage: "target os",
			},
			&cli.StringFlag{
				Name:  "arch",
				Usage: "target architecture",
			},
		},
		Action: runPull,
	}
}

func runPull(ctx *cli.Context) error {
	// image
	image := ctx.Args().Get(0)
	// option
	opt_os := ctx.String("os")
	opt_arch := ctx.String("arch")

	service := imageservice.NewServiceImagePull()
	if err := service.Pull(
		imageservice.ServiceImagePullModel{
			Image: image,
			Os:    opt_os,
			Arch:  opt_arch,
		},
	); err != nil {
		return err
	}

	fmt.Printf("image: %s pull completed\n", image)

	return nil
}
