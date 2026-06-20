package containercommand

import (
	"raind/internal/raind/core/container"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:  "ls",
		Usage: "list containers",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "all",
				Aliases: []string{"a"},
				Usage:   "list all containers",
				Value:   false,
			},
			&cli.BoolFlag{
				Name:    "include-pod",
				Aliases: []string{"p"},
				Usage:   "include pod members",
				Value:   false,
			},
		},
		Action: runList,
	}
}

func runList(ctx *cli.Context) error {
	opt_all := ctx.Bool("all")
	opt_include_pod := ctx.Bool("include-pod")

	service := container.NewServiceContainerList()
	if err := service.List(
		container.ServiceListModel{
			ListAll:    opt_all,
			IncludePod: opt_include_pod,
		},
	); err != nil {
		return err
	}
	return nil
}
