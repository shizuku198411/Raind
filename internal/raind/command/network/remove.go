package network

import (
	"fmt"
	"raind/internal/raind/core/network"

	"github.com/urfave/cli/v2"
)

func CommandRemove() *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "remove a network",
		ArgsUsage: "<network-name>",
		Action:    runRemove,
	}
}

func runRemove(ctx *cli.Context) error {
	// rtrieve network name
	bridge := ctx.Args().Get(0)
	if bridge == "" {
		return fmt.Errorf("network name required\nuse 'raind network rm <network-name>'")
	}

	service := network.NewServiceNetworkRemove()
	res, err := service.Remove(
		network.ServiceNetworkRemoveModel{
			Bridge: bridge,
		},
	)
	if err != nil {
		return err
	}

	fmt.Println(res)

	return nil
}
