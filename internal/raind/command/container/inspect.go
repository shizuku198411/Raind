package containercommand

import (
	"fmt"
	"raind/internal/raind/core/container"

	"github.com/urfave/cli/v2"
)

func CommandInspect() *cli.Command {
	return &cli.Command{
		Name:      "inspect",
		Usage:     "inspect a container",
		ArgsUsage: "<id|name>",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "json",
				Usage: "print inspect result as JSON",
				Value: false,
			},
		},
		Action: runInspect,
	}
}

func runInspect(ctx *cli.Context) error {
	target := ""
	asJSON := ctx.Bool("json")
	for _, arg := range ctx.Args().Slice() {
		switch arg {
		case "--json":
			asJSON = true
		default:
			if target == "" {
				target = arg
			} else {
				return fmt.Errorf("unexpected argument: %s", arg)
			}
		}
	}
	if target == "" {
		return fmt.Errorf("container id or name is required")
	}

	service := container.NewServiceContainerInspect()
	return service.Inspect(target, asJSON)
}
