package namespace

import (
	"fmt"
	corenamespace "raind/internal/raind/core/namespace"

	"github.com/urfave/cli/v2"
)

func CommandShow() *cli.Command {
	return &cli.Command{
		Name:      "show",
		Usage:     "show resource namespace",
		ArgsUsage: "<namespace>",
		Action: func(ctx *cli.Context) error {
			name := ctx.Args().Get(0)
			if name == "" {
				return fmt.Errorf("namespace name required")
			}
			return corenamespace.NewServiceNamespaceDetail().Detail(name)
		},
	}
}
