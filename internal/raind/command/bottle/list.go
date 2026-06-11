package bottle

import (
	"raind/internal/raind/core/bottle"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:   "ls",
		Usage:  "list bottles",
		Action: runList,
	}
}

func runList(ctx *cli.Context) error {
	service := bottle.NewServiceBottleList()
	if err := service.List(); err != nil {
		return err
	}
	return nil
}
