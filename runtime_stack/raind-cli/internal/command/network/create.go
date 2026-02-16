package network

import (
	"fmt"
	"raind/internal/core/network"

	"github.com/urfave/cli/v2"
)

func CommandCreate() *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "create a network",
		ArgsUsage: "<network-name>",
		Action:    runCreate,
	}
}

func runCreate(ctx *cli.Context) error {
	// rtrieve network name
	bridge := ctx.Args().Get(0)
	if bridge == "" {
		return fmt.Errorf("network name required\nuse 'raind network create <network-name>'")
	}

	service := network.NewServiceNetworkCreate()
	res, err := service.Create(
		network.ServiceNetworkCreateModel{
			Bridge: bridge,
		},
	)
	if err != nil {
		return err
	}

	fmt.Printf("network: %s created\n", res)

	return nil
}
