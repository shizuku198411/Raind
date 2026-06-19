package configmapcommand

import (
	"fmt"
	"raind/internal/raind/core/configmap"

	"github.com/urfave/cli/v2"
)

func CommandRemove() *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "remove a configmap",
		ArgsUsage: "<configmap-id|name>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "namespace for name lookup"},
		},
		Action: func(ctx *cli.Context) error {
			idOrName := ctx.Args().Get(0)
			if idOrName == "" {
				return fmt.Errorf("configmap id or name is required")
			}
			info, err := configmap.NewServiceRemove().Remove(idOrName, ctx.String("namespace"))
			if err != nil {
				return err
			}
			fmt.Printf("configmap: %s removed\n", info.ConfigMapId)
			return nil
		},
	}
}
