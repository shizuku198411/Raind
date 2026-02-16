package containercommand

import (
	"raind/internal/core/container"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:   "ls",
		Usage:  "list containers",
		Action: runList,
	}
}

func runList(ctx *cli.Context) error {
	service := container.NewServiceContainerList()
	if err := service.List(); err != nil {
		return err
	}
	return nil
}
