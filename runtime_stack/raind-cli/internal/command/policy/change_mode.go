package policy

import (
	"fmt"
	"raind/internal/core/policy"

	"github.com/urfave/cli/v2"
)

func CommandChangeMode() *cli.Command {
	return &cli.Command{
		Name:      "ns-mode",
		Usage:     "change ns mode",
		ArgsUsage: "<mode (observe|enforce)>",
		Action:    runChangeMode,
	}
}

func runChangeMode(ctx *cli.Context) error {
	// mode
	mode := ctx.Args().Get(0)
	if mode != "observe" && mode != "enforce" {
		return fmt.Errorf("invalid mode: %s. expect: observe|enforce", mode)
	}

	service := policy.NewServicePolicyChangeMode()
	err := service.ChangeMode(
		policy.ServiceChangeModeModel{
			Mode: mode,
		},
	)
	if err != nil {
		return err
	}

	return nil
}
