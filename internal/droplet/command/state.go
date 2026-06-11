package command

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"raind/internal/droplet/status"
)

func commandState() *cli.Command {
	return &cli.Command{
		Name:      "state",
		Usage:     "query state a container",
		ArgsUsage: "<container-id>",
		Action:    runState,
	}
}

func runState(ctx *cli.Context) error {
	// retrieve container id
	containerId := ctx.Args().Get(0)

	containerStatusHandler := status.NewStatusHandler()
	statusInfo, err := containerStatusHandler.ReadStatusFile(containerId)
	if err != nil {
		return err
	}

	fmt.Println(statusInfo)

	return nil
}
