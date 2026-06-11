package servicecommand

import (
	"fmt"
	"raind/internal/raind/core/service"

	"github.com/urfave/cli/v2"
)

func CommandShow() *cli.Command {
	return &cli.Command{
		Name:      "show",
		Usage:     "show a service detail",
		ArgsUsage: "<service-id>",
		Action:    runShow,
	}
}

func runShow(ctx *cli.Context) error {
	id := ctx.Args().Get(0)
	if id == "" {
		return fmt.Errorf("service id is required")
	}

	svc := service.NewServiceServiceDetail()
	if err := svc.Detail(id); err != nil {
		return err
	}

	return nil
}
