package policy

import (
	"fmt"
	"raind/internal/core/policy"

	"github.com/urfave/cli/v2"
)

func CommandCreate() *cli.Command {
	return &cli.Command{
		Name:  "add",
		Usage: "add a policy",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "type",
				Usage:    "policy type (ew/ns-obs/ns-enf)",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "source",
				Aliases:  []string{"s"},
				Usage:    "source container name",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "destination",
				Aliases:  []string{"d"},
				Usage:    "destination container name",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "protocol",
				Aliases: []string{"p"},
				Usage:   "protocol",
			},
			&cli.IntFlag{
				Name:  "dport",
				Usage: "destination port",
			},
			&cli.StringFlag{
				Name:  "comment",
				Usage: "policy comment",
			},
		},
		Action: runCreate,
	}
}

func runCreate(ctx *cli.Context) error {
	// option
	opt_type, err := validateType(ctx.String("type"))
	if err != nil {
		return err
	}
	opt_source := ctx.String("source")
	opt_dest := ctx.String("destination")
	opt_protocol := ctx.String("protocol")
	opt_dport := ctx.Int("dport")
	opt_comment := ctx.String("comment")

	service := policy.NewServicePolicyCreate()
	err = service.Create(
		policy.ServiceCreateModel{
			Chain:       opt_type,
			Source:      opt_source,
			Destination: opt_dest,
			Protocol:    opt_protocol,
			DestPort:    opt_dport,
			Comment:     opt_comment,
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func validateType(v string) (string, error) {
	if v != "ew" && v != "ns-obs" && v != "ns-enf" {
		return "", fmt.Errorf("invalid type: %s. allowed type: es/ns-obs/ns-enf")
	}
	return v, nil
}
