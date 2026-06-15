package command

import (
	"fmt"

	"raind/internal/droplet/spec"

	"github.com/urfave/cli/v2"
)

func rootlessOptionsFromSpecCLI(ctx *cli.Context) (bool, string, error) {
	rootless := ctx.Bool("rootless")
	mode := ctx.String("rootless-mode")

	switch mode {
	case "", spec.RootlessModeShiftedRoot:
		mode = spec.RootlessModeShiftedRoot
	case spec.RootlessModeLoginRoot:
		rootless = true
	default:
		return false, "", fmt.Errorf("invalid --rootless-mode: %s, expected %s or %s", mode, spec.RootlessModeShiftedRoot, spec.RootlessModeLoginRoot)
	}

	if ctx.IsSet("rootless-mode") {
		rootless = true
	}

	return rootless, mode, nil
}
