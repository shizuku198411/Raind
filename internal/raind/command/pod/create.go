package podcommand

import (
	"fmt"
	"raind/internal/raind/core/pod"
	"strings"

	"github.com/urfave/cli/v2"
)

func CommandCreate() *cli.Command {
	return &cli.Command{
		Name:  "create",
		Usage: "create a pod",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "name",
				Aliases:  []string{"n"},
				Usage:    "pod name",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "namespace",
				Usage: "pod namespace",
				Value: "default",
			},
			&cli.StringFlag{
				Name:  "uid",
				Usage: "pod uid",
			},
			&cli.StringSliceFlag{
				Name:    "label",
				Aliases: []string{"l"},
				Usage:   "labels (key=value)",
			},
			&cli.StringSliceFlag{
				Name:    "annotation",
				Aliases: []string{"a"},
				Usage:   "annotations (key=value)",
			},
		},
		Action: runCreate,
	}
}

func runCreate(ctx *cli.Context) error {
	labels, err := parseKeyValueSlice(ctx.StringSlice("label"))
	if err != nil {
		return err
	}
	annotations, err := parseKeyValueSlice(ctx.StringSlice("annotation"))
	if err != nil {
		return err
	}

	service := pod.NewServicePodCreate()
	podId, err := service.Create(
		pod.ServicePodCreateModel{
			Name:        ctx.String("name"),
			Namespace:   ctx.String("namespace"),
			UID:         ctx.String("uid"),
			Labels:      labels,
			Annotations: annotations,
		},
	)
	if err != nil {
		return err
	}

	if podId == "" {
		fmt.Printf("pod created\n")
		return nil
	}
	fmt.Printf("pod: %s created\n", podId)
	return nil
}

func parseKeyValueSlice(items []string) (map[string]string, error) {
	if len(items) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(items))
	for _, item := range items {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, fmt.Errorf("invalid format: %s, required format: key=value", item)
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		out[key] = value
	}
	return out, nil
}
