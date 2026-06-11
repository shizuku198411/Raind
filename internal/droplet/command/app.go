package command

import (
	"github.com/urfave/cli/v2"
	"raind/internal/droplet/buildinfo"
)

func NewApp() *cli.App {
	app := &cli.App{
		Name:    "droplet",
		Usage:   "low-level container runtime",
		Version: buildinfo.Version,
		Commands: []*cli.Command{
			commandCreate(),
			commandStart(),
			commandKill(),
			commandDelete(),
			commandState(),
			commandRun(),
			commandExec(),
			commandExecShim(),
			commandSpec(),
			commandList(),
			commandInit(),
			commandShim(),
			commandAttach(),
		},
	}

	// disable slice flag separator
	app.DisableSliceFlagSeparator = true

	return app
}
