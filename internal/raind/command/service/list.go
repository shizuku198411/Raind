package servicecommand

import (
	"raind/internal/raind/core/service"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:  "ls",
		Usage: "list services",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "filter by namespace"},
		},
		Action: runList,
	}
}

func runList(ctx *cli.Context) error {
	svc := service.NewServiceServiceList()
	if err := svc.List(ctx.String("namespace")); err != nil {
		return err
	}
	return nil
}
