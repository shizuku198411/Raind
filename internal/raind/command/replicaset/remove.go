package replicasetcommand

import (
	"fmt"
	"raind/internal/raind/core/replicaset"

	"github.com/urfave/cli/v2"
)

func CommandRemove() *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "remove a replicaset",
		Aliases:   []string{"delete"},
		ArgsUsage: "<replicaset-id>",
		Action:    runRemove,
	}
}

func runRemove(ctx *cli.Context) error {
	id := ctx.Args().Get(0)
	if id == "" {
		return fmt.Errorf("replicaset id is required")
	}

	service := replicaset.NewServiceReplicaSetRemove()
	removedId, err := service.Remove(
		replicaset.ServiceReplicaSetRemoveModel{
			Id: id,
		},
	)
	if err != nil {
		return err
	}

	fmt.Printf("replicaset: %s removed\n", removedId)
	return nil
}
