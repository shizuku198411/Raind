package servicecommand

import (
	"fmt"
	"raind/internal/raind/core/service"

	"github.com/urfave/cli/v2"
)

func CommandRemove() *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "remove a service",
		ArgsUsage: "<service-id>",
		Action:    runRemove,
	}
}

func runRemove(ctx *cli.Context) error {
	id := ctx.Args().Get(0)
	if id == "" {
		return fmt.Errorf("service id is required")
	}

	svc := service.NewServiceServiceRemove()
	removedId, err := svc.Remove(
		service.ServiceServiceRemoveModel{
			Id: id,
		},
	)
	if err != nil {
		return err
	}

	fmt.Printf("service: %s removed\n", removedId)
	return nil
}
