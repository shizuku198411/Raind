package policy

import (
	"raind/internal/core/policy"

	"github.com/urfave/cli/v2"
)

func CommandRemove() *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "remove policy",
		ArgsUsage: "<policy-id>",
		Action:    runRemove,
	}
}

func runRemove(ctx *cli.Context) error {
	// option
	policyId := ctx.Args().Get(0)

	service := policy.NewServicePolicyRemove()
	if err := service.Remove(
		policy.RemoveRequestModel{
			Id: policyId,
		},
	); err != nil {
		return err
	}

	return nil
}
