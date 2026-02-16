package bottle

import (
	"fmt"
	"raind/internal/core/bottle"

	"github.com/urfave/cli/v2"
)

func CommandDelete() *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "remove a bottle",
		ArgsUsage: "<bottleId or bottleName>",
		Action:    runDelete,
	}
}

func runDelete(ctx *cli.Context) error {
	target := ctx.Args().Get(0)
	if target == "" {
		return fmt.Errorf("bottle id or name is required")
	}

	service := bottle.NewServiceBottleDelete()
	if err := service.Delete(
		bottle.ServiceBottleDeleteModel{
			Target: target,
		},
	); err != nil {
		return err
	}

	return nil
}
