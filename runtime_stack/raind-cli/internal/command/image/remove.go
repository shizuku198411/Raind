package imagecommand

import (
	"fmt"
	imageservice "raind/internal/core/image"

	"github.com/urfave/cli/v2"
)

func CommandRemove() *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "remove an image",
		ArgsUsage: "<image:tag>",
		Action:    runRemove,
	}
}

func runRemove(ctx *cli.Context) error {
	// image
	image := ctx.Args().Get(0)

	service := imageservice.NewServiceImageRemove()
	if err := service.Remove(
		imageservice.ServiceImageRemoveModel{
			Image: image,
		},
	); err != nil {
		return err
	}

	fmt.Printf("image: %s remove completed\n", image)

	return nil
}
