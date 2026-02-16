package policy

import (
	"raind/internal/core/policy"

	"github.com/urfave/cli/v2"
)

func CommandRevert() *cli.Command {
	return &cli.Command{
		Name:   "revert",
		Usage:  "revert policy",
		Action: runRevert,
	}
}

func runRevert(ctx *cli.Context) error {
	service := policy.NewServicePolicyRevert()
	err := service.Revert()
	if err != nil {
		return err
	}

	return nil
}
