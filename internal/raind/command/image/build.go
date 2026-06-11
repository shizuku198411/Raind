package imagecommand

import (
	"fmt"
	imageservice "raind/internal/raind/core/image"

	"github.com/urfave/cli/v2"
)

func CommandBuild() *cli.Command {
	return &cli.Command{
		Name:  "build",
		Usage: "build an image from a Dripfile context",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "file",
				Aliases:  []string{"f"},
				Usage:    "build context directory (contains Dripfile)",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "tag",
				Aliases:  []string{"t"},
				Usage:    "image tag (e.g. repo/name:tag)",
				Required: true,
			},
		},
		Action: runBuild,
	}
}

func runBuild(ctx *cli.Context) error {
	contextDir := ctx.String("file")
	tag := ctx.String("tag")

	if contextDir == "" {
		return fmt.Errorf("missing required flag: -f, --file (context directory)")
	}
	if tag == "" {
		return fmt.Errorf("missing required flag: -t, --tag (image:tag)")
	}

	fmt.Printf("image: %s build start\n", tag)

	service := imageservice.NewServiceImageBuild()
	if err := service.Build(
		imageservice.ServiceImageBuildModel{
			ContextDir: contextDir,
			Tag:        tag,
		},
	); err != nil {
		return err
	}

	fmt.Printf("image: %s build completed\n", tag)

	return nil
}
