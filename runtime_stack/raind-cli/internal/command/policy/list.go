package policy

import (
	"raind/internal/core/policy"

	"github.com/urfave/cli/v2"
)

func CommandList() *cli.Command {
	return &cli.Command{
		Name:  "ls",
		Usage: "list policies",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "type",
				Usage: "policy type (ew/ns-obs/ns-enf)",
			},
		},
		Action: runList,
	}
}

func runList(ctx *cli.Context) error {
	// option
	opt_type := ctx.String("type")
	if opt_type != "" {
		got, err := validateType(ctx.String("type"))
		if err != nil {
			return err
		}
		opt_type = got
	}

	service := policy.NewServicePolicyList()
	err := service.List(
		policy.ListRequestModel{
			ChainName: opt_type,
		},
	)
	if err != nil {
		return err
	}

	return nil
}
