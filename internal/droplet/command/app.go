package command

import (
	"os"

	"github.com/urfave/cli/v2"
	"raind/internal/droplet/buildinfo"
)

func NewApp() *cli.App {
	app := &cli.App{
		Name:    "droplet",
		Usage:   "low-level container runtime",
		Version: buildinfo.Version,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "root",
				Usage: "root directory for runtime state",
			},
			&cli.StringFlag{
				Name:  "log",
				Usage: "path to an OCI runtime error log file",
			},
			&cli.StringFlag{
				Name:  "log-format",
				Usage: "OCI runtime log format",
			},
			&cli.BoolFlag{
				Name:  "systemd-cgroup",
				Usage: "accept runc-compatible systemd cgroup flag",
			},
		},
		Before: func(ctx *cli.Context) error {
			if root := ctx.String("root"); root != "" {
				return os.Setenv("RAIND_ROOT_DIR", root)
			}
			return nil
		},
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
