package command

import (
	"droplet/internal/container"

	"github.com/urfave/cli/v2"
)

func commandCreate() *cli.Command {
	return &cli.Command{
		Name:      "create",
		Usage:     "create a container",
		ArgsUsage: "<container-id>",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:   "print-pid",
				Hidden: true,
				Value:  false,
			},
			&cli.BoolFlag{
				Name:    "tty",
				Aliases: []string{"t"},
				Value:   false,
			},
		},
		Action: runCreate,
	}
}

func runCreate(ctx *cli.Context) error {
	// retrieve container ID
	containerId := ctx.Args().Get(0)
	pidPrintFlag := ctx.Bool("print-pid")
	ttyFlag := ctx.Bool("tty")

	containerCreator := container.NewContainerCreator()
	err := containerCreator.Create(
		container.CreateOption{
			ContainerId:  containerId,
			PrintPidFlag: pidPrintFlag,
			TtyFlag:      ttyFlag,
		},
	)

	if err != nil {
		return err
	}

	return nil
}
