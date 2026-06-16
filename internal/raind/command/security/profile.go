package security

import (
	core "raind/internal/raind/core/securityprofile"

	"github.com/urfave/cli/v2"
)

func CommandProfile() *cli.Command {
	return &cli.Command{
		Name:  "profile",
		Usage: "security profile operation",
		Subcommands: []*cli.Command{
			commandProfileList(),
			commandProfileShow(),
		},
	}
}

func commandProfileList() *cli.Command {
	return &cli.Command{
		Name:    "ls",
		Aliases: []string{"list"},
		Usage:   "list security profiles",
		Action: func(ctx *cli.Context) error {
			return core.NewServiceList().List()
		},
	}
}

func commandProfileShow() *cli.Command {
	return &cli.Command{
		Name:      "show",
		Usage:     "show security profile detail",
		ArgsUsage: "<name>",
		Action: func(ctx *cli.Context) error {
			return core.NewServiceShow().Show(ctx.Args().First())
		},
	}
}
