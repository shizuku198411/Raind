package configmapcommand

import (
	"fmt"
	"raind/internal/raind/core/configmap"

	"github.com/urfave/cli/v2"
)

func CommandShow() *cli.Command {
	return &cli.Command{
		Name:      "show",
		Usage:     "show a configmap detail",
		ArgsUsage: "<configmap-id|name>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "namespace for name lookup"},
		},
		Action: func(ctx *cli.Context) error {
			idOrName := ctx.Args().Get(0)
			if idOrName == "" {
				return fmt.Errorf("configmap id or name is required")
			}
			return configmap.NewServiceDetail().Detail(idOrName, ctx.String("namespace"))
		},
	}
}
