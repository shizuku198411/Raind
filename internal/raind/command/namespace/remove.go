package namespace

import (
	"fmt"
	corenamespace "raind/internal/raind/core/namespace"

	"github.com/urfave/cli/v2"
)

func CommandRemove() *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Aliases:   []string{"delete"},
		Usage:     "remove resource namespace",
		ArgsUsage: "<namespace>",
		Action: func(ctx *cli.Context) error {
			name := ctx.Args().Get(0)
			if name == "" {
				return fmt.Errorf("namespace name required")
			}
			deleted, err := corenamespace.NewServiceNamespaceRemove().Remove(name)
			if err != nil {
				return err
			}
			fmt.Printf("namespace: %s removed\n", deleted)
			return nil
		},
	}
}
