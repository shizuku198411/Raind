package secretcommand

import (
	"fmt"
	"raind/internal/raind/core/secret"

	"github.com/urfave/cli/v2"
)

func CommandShow() *cli.Command {
	return &cli.Command{
		Name:      "show",
		Usage:     "show a secret detail without revealing values",
		ArgsUsage: "<secret-id|name>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "namespace for name lookup"},
		},
		Action: func(ctx *cli.Context) error {
			idOrName := ctx.Args().Get(0)
			if idOrName == "" {
				return fmt.Errorf("secret id or name is required")
			}
			return secret.NewServiceDetail().Detail(idOrName, ctx.String("namespace"))
		},
	}
}
