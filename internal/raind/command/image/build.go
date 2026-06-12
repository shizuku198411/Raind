package imagecommand

import (
	"fmt"
	imageservice "raind/internal/raind/core/image"

	"github.com/urfave/cli/v2"
)

func CommandBuild() *cli.Command {
	return &cli.Command{
		Name:  "build",
		Usage: "build an image from a Dockerfile or Dripfile context",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "file",
				Aliases:  []string{"f"},
				Usage:    "build context directory (contains Dockerfile or Dripfile)",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "tag",
				Aliases:  []string{"t"},
				Usage:    "image tag (e.g. repo/name:tag)",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "dockerfile",
				Aliases: []string{"D"},
				Usage:   "Dockerfile/Dripfile path inside the context",
			},
			&cli.StringFlag{
				Name:  "dripfile",
				Usage: "Dripfile path inside the context",
			},
		},
		Action: runBuild,
	}
}

func runBuild(ctx *cli.Context) error {
	contextDir := ctx.String("file")
	tag := ctx.String("tag")
	buildFile := ctx.String("dockerfile")
	if ctx.String("dripfile") != "" {
		if buildFile != "" {
			return fmt.Errorf("--dockerfile and --dripfile cannot be used together")
		}
		buildFile = ctx.String("dripfile")
	}

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
			Dripfile:   buildFile,
		},
	); err != nil {
		return err
	}

	fmt.Printf("image: %s build completed\n", tag)

	return nil
}
