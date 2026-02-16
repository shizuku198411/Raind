package command

import (
	bottlecommand "raind/internal/command/bottle"
	completioncommand "raind/internal/command/completion"
	containercommand "raind/internal/command/container"
	imagecommand "raind/internal/command/image"
	logscommand "raind/internal/command/logs"
	networkcommand "raind/internal/command/network"
	policycommand "raind/internal/command/policy"
	resourcecommand "raind/internal/command/resource"

	"github.com/urfave/cli/v2"
)

func NewApp() *cli.App {
	app := &cli.App{
		Name:  "raind",
		Usage: "raind container runtime",
		Commands: []*cli.Command{
			{
				Name:  "container",
				Usage: "container operation",
				Subcommands: []*cli.Command{
					containercommand.CommandCreate(),
					containercommand.CommandStart(),
					containercommand.CommandStop(),
					containercommand.CommandRemove(),
					containercommand.CommandList(),
					containercommand.CommandAttach(),
					containercommand.CommandRun(),
					containercommand.CommandExec(),
					containercommand.CommandLogs(),
				},
			},
			{
				Name:  "bottle",
				Usage: "bottle operation",
				Subcommands: []*cli.Command{
					bottlecommand.CommandCreate(),
					bottlecommand.CommandStart(),
					bottlecommand.CommandStop(),
					bottlecommand.CommandDelete(),
					bottlecommand.CommandList(),
					bottlecommand.CommandShow(),
				},
			},
			{
				Name:  "image",
				Usage: "image operation",
				Subcommands: []*cli.Command{
					imagecommand.CommandPull(),
					imagecommand.CommandBuild(),
					imagecommand.CommandList(),
					imagecommand.CommandRemove(),
				},
			},
			{
				Name:  "network",
				Usage: "network operation",
				Subcommands: []*cli.Command{
					networkcommand.CommandList(),
					networkcommand.CommandCreate(),
					networkcommand.CommandRemove(),
				},
			},
			{
				Name:  "resource",
				Usage: "resource operation",
				Subcommands: []*cli.Command{
					resourcecommand.CommandApply(),
					resourcecommand.CommandRemove(),
					resourcecommand.CommandPod(),
					resourcecommand.CommandReplicaSet(),
					resourcecommand.CommandService(),
				},
			},
			{
				Name:  "policy",
				Usage: "policy operation",
				Subcommands: []*cli.Command{
					policycommand.CommandCreate(),
					policycommand.CommandList(),
					policycommand.CommandCommit(),
					policycommand.CommandRemove(),
					policycommand.CommandRevert(),
					policycommand.CommandChangeMode(),
				},
			},
			{
				Name:  "logs",
				Usage: "log operation",
				Subcommands: []*cli.Command{
					logscommand.CommandLogs(),
				},
			},
		},
	}

	// disable slice flag separator
	app.DisableSliceFlagSeparator = true
	app.Commands = append(app.Commands, completioncommand.CommandCompletion(app))

	return app
}
