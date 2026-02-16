package bottle

import (
	"fmt"
	"raind/internal/core/bottle"

	"github.com/urfave/cli/v2"
)

func CommandStop() *cli.Command {
	return &cli.Command{
		Name:      "stop",
		Usage:     "stop a bottle",
		ArgsUsage: "<bottleId or bottleName>",
		Action:    runStop,
	}
}

func runStop(ctx *cli.Context) error {
	target := ctx.Args().Get(0)
	if target == "" {
		return fmt.Errorf("bottle id or name is required")
	}

	service := bottle.NewServiceBottleStop()
	if err := service.Stop(
		bottle.ServiceBottleStopModel{
			Target: target,
		},
	); err != nil {
		return err
	}

	return nil
}
