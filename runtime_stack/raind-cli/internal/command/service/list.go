package servicecommand

import (
	"raind/internal/core/service"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:   "ls",
		Usage:  "list services",
		Action: runList,
	}
}

func runList(ctx *cli.Context) error {
	svc := service.NewServiceServiceList()
	if err := svc.List(); err != nil {
		return err
	}
	return nil
}
