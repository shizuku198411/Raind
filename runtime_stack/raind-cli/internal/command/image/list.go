package imagecommand

import (
	imageservice "raind/internal/core/image"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:   "ls",
		Usage:  "list images",
		Action: runList,
	}
}

func runList(ctx *cli.Context) error {
	service := imageservice.NewServiceImageList()
	if err := service.List(); err != nil {
		return err
	}
	return nil
}
