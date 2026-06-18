package resourcecommand

import (
	watchcommand "raind/internal/raind/command/watch"

	"github.com/urfave/cli/v2"
)

func runWaitOrHelp(ctx *cli.Context, list func() error) error {
	if !watchcommand.Enabled(ctx) {
		return cli.ShowSubcommandHelp(ctx)
	}
	return watchcommand.Run(true, list)
}
