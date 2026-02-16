package replicasetcommand

import (
	"fmt"
	"raind/internal/core/replicaset"

	"github.com/urfave/cli/v2"
)

func CommandShow() *cli.Command {
	return &cli.Command{
		Name:      "show",
		Usage:     "show a replicaset detail",
		Aliases:   []string{"describe"},
		ArgsUsage: "<replicaset-id>",
		Action:    runShow,
	}
}

func runShow(ctx *cli.Context) error {
	id := ctx.Args().Get(0)
	if id == "" {
		return fmt.Errorf("replicaset id is required")
	}

	service := replicaset.NewServiceReplicaSetDetail()
	if err := service.Detail(id); err != nil {
		return err
	}

	return nil
}
