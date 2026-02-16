package bottle

import (
	"fmt"
	"raind/internal/core/bottle"

	"github.com/urfave/cli/v2"
)

func CommandShow() *cli.Command {
	return &cli.Command{
		Name:      "show",
		Usage:     "show a bottle detail",
		ArgsUsage: "<bottleId or bottleName>",
		Action:    runShow,
	}
}

func runShow(ctx *cli.Context) error {
	target := ctx.Args().Get(0)
	if target == "" {
		return fmt.Errorf("bottle id or name is required")
	}

	service := bottle.NewServiceBottleDetail()
	if err := service.Detail(target); err != nil {
		return err
	}

	return nil
}
