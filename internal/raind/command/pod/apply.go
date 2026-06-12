package podcommand

import (
	"fmt"
	"raind/internal/raind/core/pod"

	"github.com/urfave/cli/v2"
)

func CommandApply() *cli.Command {
	return &cli.Command{
		Name:  "apply",
		Usage: "apply a pod definition from yaml",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "file",
				Aliases:  []string{"f"},
				Usage:    "pod definition yaml file",
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
	result, err := service.Apply(
		pod.ServicePodApplyModel{
			FilePath: yamlPath,
		},
	)
	if err != nil {
		return err
	}

	if len(result.Pods) == 0 {
		fmt.Printf("pod applied\n")
		return nil
	}
	for _, p := range result.Pods {
		fmt.Printf("pod: %s applied\n", p.PodId)
	}
	return nil
}
