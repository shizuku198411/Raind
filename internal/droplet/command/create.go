package command

import (
	"raind/internal/droplet/container"

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
			&cli.StringFlag{
				Name:    "bundle",
				Aliases: []string{"b"},
				Usage:   "path to an OCI bundle directory",
			},
			&cli.StringFlag{
				Name:  "console-socket",
				Usage: "path to an AF_UNIX socket that receives the terminal console file descriptor",
			},
			&cli.StringFlag{
				Name:  "pid-file",
				Usage: "write the container init pid to this file",
			},
		},
		Action: runCreate,
	}
}

func runCreate(ctx *cli.Context) error {
	// retrieve container ID
	containerId := ctx.Args().Get(0)
	if containerId == "" {
		return cli.Exit("container-id is required", 1)
	}
	pidPrintFlag := ctx.Bool("print-pid")
	ttyFlag := ctx.Bool("tty")
	bundle := ctx.String("bundle")
	consoleSocket := ctx.String("console-socket")
	pidFile := ctx.String("pid-file")

	containerCreator := container.NewContainerCreator()
	err := containerCreator.Create(
		container.CreateOption{
			ContainerId:   containerId,
			Bundle:        bundle,
			ConsoleSocket: consoleSocket,
			PidFile:       pidFile,
			PrintPidFlag:  pidPrintFlag,
			TtyFlag:       ttyFlag,
		},
	)

	if err != nil {
		return err
	}

	return nil
}
