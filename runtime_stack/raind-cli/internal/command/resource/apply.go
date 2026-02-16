package resourcecommand

import (
	"fmt"
	"raind/internal/core/pod"

	"github.com/urfave/cli/v2"
)

func CommandApply() *cli.Command {
	return &cli.Command{
		Name:  "apply",
		Usage: "apply resources from yaml",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "file",
				Aliases:  []string{"f"},
				Usage:    "resource definition yaml file",
				Required: true,
			},
		},
		Action: runApply,
	}
}

func runApply(ctx *cli.Context) error {
	yamlPath := ctx.String("file")
	if yamlPath == "" {
		return fmt.Errorf("missing required flag: -f, --file (yaml file)")
	}

	service := pod.NewServicePodApply()
	resourceId, err := service.Apply(
		pod.ServicePodApplyModel{
			FilePath: yamlPath,
		},
	)
	if err != nil {
		return err
	}

	if resourceId == "" {
		fmt.Printf("resource applied\n")
		return nil
	}
	fmt.Printf("resource: %s applied\n", resourceId)
	return nil
}
