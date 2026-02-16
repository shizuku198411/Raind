package containercommand

import (
	"fmt"
	"raind/internal/core/container"

	"github.com/urfave/cli/v2"
)

func CommandLogs() *cli.Command {
	return &cli.Command{
		Name:      "logs",
		Usage:     "view container logs",
		ArgsUsage: "<container-id>",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "line",
				Usage: "max line number",
			},
			&cli.BoolFlag{
				Name:  "pager",
				Usage: "open log with pager",
			},
		},
		Action: runLog,
	}
}

func runLog(ctx *cli.Context) error {
	// container ID
	containerId := ctx.Args().Get(0)
	if containerId == "" {
		return fmt.Errorf("container-id required")
	}
	// option
	opt_tailLine := ctx.Int("line")
	opt_pager := ctx.Bool("pager")

	service := container.NewServiceContainerLog()
	if err := service.GetLog(container.ServiceLogsModel{
		ContainerId: containerId,
		TailLine:    opt_tailLine,
		Pager:       opt_pager,
	}); err != nil {
		return err
	}
	return nil
}
