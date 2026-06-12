package imagecommand

import (
	"fmt"
	"os"
	imageservice "raind/internal/raind/core/image"

	"github.com/urfave/cli/v2"
)

func CommandBuild() *cli.Command {
	return &cli.Command{
		Name:      "build",
		Usage:     "build an image from a Dockerfile or Dripfile context",
		ArgsUsage: "<context-path>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "file",
				Aliases: []string{"f"},
				Usage:   "Dockerfile/Dripfile path inside the context",
			},
			&cli.StringFlag{
				Name:     "tag",
				Aliases:  []string{"t"},
				Usage:    "image tag (e.g. repo/name:tag)",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "dockerfile",
				Usage: "Dockerfile/Dripfile path inside the context (alias of --file)",
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
	contextDir, buildFile, legacy, err := resolveBuildArgs(ctx)
	if err != nil {
		return err
	}
	if legacy {
		fmt.Println("warning: using -f/--file as the build context is deprecated; use: raind image build -t <image:tag> <context-path>")
	}
	tag := ctx.String("tag")

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

func resolveBuildArgs(ctx *cli.Context) (contextDir string, buildFile string, legacyContext bool, err error) {
	if ctx.Args().Len() > 1 {
		return "", "", false, fmt.Errorf("too many arguments: expected one build context path")
	}

	contextDir = ctx.Args().Get(0)
	buildFile = ctx.String("file")
	if ctx.String("dockerfile") != "" {
		if buildFile != "" {
			return "", "", false, fmt.Errorf("--file and --dockerfile cannot be used together")
		}
		buildFile = ctx.String("dockerfile")
	}
	if ctx.String("dripfile") != "" {
		if buildFile != "" {
			return "", "", false, fmt.Errorf("--file, --dockerfile, and --dripfile cannot be used together")
		}
		buildFile = ctx.String("dripfile")
	}

	if contextDir != "" {
		return contextDir, buildFile, false, nil
	}
	if buildFile != "" && isDir(buildFile) {
		return buildFile, "", true, nil
	}
	return "", "", false, fmt.Errorf("missing build context path")
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
