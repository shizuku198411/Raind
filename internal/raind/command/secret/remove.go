package secretcommand

import (
	"fmt"
	"raind/internal/raind/core/secret"

	"github.com/urfave/cli/v2"
)

func CommandRemove() *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "remove a secret",
		ArgsUsage: "<secret-id|name>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "namespace for name lookup"},
		},
		Action: func(ctx *cli.Context) error {
			idOrName := ctx.Args().Get(0)
			if idOrName == "" {
				return fmt.Errorf("secret id or name is required")
			}
			info, err := secret.NewServiceRemove().Remove(idOrName, ctx.String("namespace"))
			if err != nil {
				return err
			}
			fmt.Printf("secret: %s removed\n", info.SecretId)
			return nil
		},
	}
}
