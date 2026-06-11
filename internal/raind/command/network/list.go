package network

import (
	"raind/internal/raind/core/network"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:   "ls",
		Usage:  "list networks",
		Action: runList,
	}
}

func runList(ctx *cli.Context) error {
	service := network.NewServiceNetworkList()
	if err := service.List(); err != nil {
		return err
	}
	return nil
}
