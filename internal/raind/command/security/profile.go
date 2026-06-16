package security

import (
	policycommand "raind/internal/raind/command/policy"
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
			commandProfileRegister(),
			commandProfileDelete(),
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

func commandProfileRegister() *cli.Command {
	return &cli.Command{
		Name:  "register",
		Usage: "register custom security profile",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "file",
				Aliases:  []string{"f"},
				Usage:    "security profile yaml file",
				Required: true,
			},
		},
		Action: func(ctx *cli.Context) error {
			return core.NewServiceRegister().Register(ctx.String("file"))
		},
	}
}

func commandProfileDelete() *cli.Command {
	return &cli.Command{
		Name:      "delete",
		Aliases:   []string{"rm"},
		Usage:     "delete custom security profile",
		ArgsUsage: "<name>",
		Action: func(ctx *cli.Context) error {
			return core.NewServiceDelete().Delete(ctx.Args().First())
		},
	}
}

func CommandPolicy() *cli.Command {
	return &cli.Command{
		Name:  "policy",
		Usage: "security policy operation",
		Subcommands: []*cli.Command{
			policycommand.CommandCreate(),
			policycommand.CommandList(),
			policycommand.CommandCommit(),
			policycommand.CommandRemove(),
			policycommand.CommandRevert(),
			policycommand.CommandChangeMode(),
		},
	}
}
