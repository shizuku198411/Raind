package bottle

import (
	"fmt"
	"raind/internal/raind/core/bottle"

	"github.com/urfave/cli/v2"
)

func CommandStart() *cli.Command {
	return &cli.Command{
		Name:      "start",
		Usage:     "start a bottle",
		ArgsUsage: "<bottleId or bottleName>",
		Action:    runStart,
	}
}

func runStart(ctx *cli.Context) error {
	target := ctx.Args().Get(0)
	if target == "" {
		return fmt.Errorf("bottle id or name is required")
	}

	service := bottle.NewServiceBottleStart()
	if err := service.Start(
		bottle.ServiceBottleStartModel{
			Target: target,
		},
	); err != nil {
		return err
	}

	return nil
}
