package replicasetcommand

import (
	"fmt"
	"raind/internal/core/replicaset"

	"github.com/urfave/cli/v2"
)

func CommandScale() *cli.Command {
	return &cli.Command{
		Name:      "scale",
		Usage:     "scale a replicaset",
		ArgsUsage: "<replicaset-id>",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:     "replicas",
				Aliases:  []string{"r"},
				Usage:    "number of replicas",
				Required: true,
			},
		},
		Action: runScale,
	}
}

func runScale(ctx *cli.Context) error {
	id := ctx.Args().Get(0)
	if id == "" {
		return fmt.Errorf("replicaset id is required")
	}

	replicas := ctx.Int("replicas")
	if replicas < 0 {
		return fmt.Errorf("replicas must be >= 0")
	}

	service := replicaset.NewServiceReplicaSetScale()
	data, err := service.Scale(
		replicaset.ServiceReplicaSetScaleModel{
			Id:       id,
			Replicas: replicas,
		},
	)
	if err != nil {
		return err
	}

	targetId := data.ReplicaSetId
	if targetId == "" {
		targetId = id
	}
	fmt.Printf("replicaset: %s scaled to %d\n", targetId, data.Replicas)
	return nil
}
