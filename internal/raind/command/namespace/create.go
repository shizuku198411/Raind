package namespace

import (
	"fmt"
	corenamespace "raind/internal/raind/core/namespace"

	"github.com/urfave/cli/v2"
)

func CommandCreate() *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "create a resource namespace",
		ArgsUsage: "<namespace>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "network",
				Usage: "bind namespace to an existing network",
			},
		},
		Action: runCreate,
	}
}

func runCreate(ctx *cli.Context) error {
	name := ctx.Args().Get(0)
	if name == "" {
		return fmt.Errorf("namespace name required")
	}
	service := corenamespace.NewServiceNamespaceCreate()
	info, err := service.Create(corenamespace.CreateModel{Name: name, Network: ctx.String("network")})
	if err != nil {
		return err
	}
	fmt.Printf("namespace: %s created (network: %s)\n", info.Name, info.Network)
	return nil
}
