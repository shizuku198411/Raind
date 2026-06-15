package containercommand

import (
	"fmt"
	"os"
	"strconv"

	"github.com/urfave/cli/v2"
)

const (
	rootlessModeShiftedRoot = "shifted-root"
	rootlessModeLoginRoot   = "login-root"
)

func rootlessOptionsFromCLI(ctx *cli.Context) (bool, string, int, int, error) {
	rootless := ctx.Bool("rootless")
	mode := ctx.String("rootless-mode")

	switch mode {
	case "", rootlessModeShiftedRoot:
		mode = rootlessModeShiftedRoot
	case rootlessModeLoginRoot:
		// Specifying a rootless mapping mode is meaningful only together with
		// rootless execution. Treat it as an explicit request for rootless so
		// `--rootless-mode login-root` does not silently run as a rootful
		// container.
		rootless = true
	default:
		return false, "", 0, 0, fmt.Errorf("invalid --rootless-mode: %s, expected %s or %s", mode, rootlessModeShiftedRoot, rootlessModeLoginRoot)
	}

	if ctx.IsSet("rootless-mode") {
		rootless = true
	}

	rootUID, rootGID := loginRootIDsFromEnv()
	return rootless, mode, rootUID, rootGID, nil
}

func loginRootIDsFromEnv() (int, int) {
	uid := os.Getuid()
	gid := os.Getgid()

	if sudoUID := os.Getenv("SUDO_UID"); sudoUID != "" {
		if parsed, err := strconv.Atoi(sudoUID); err == nil && parsed >= 0 {
			uid = parsed
		}
	}
	if sudoGID := os.Getenv("SUDO_GID"); sudoGID != "" {
		if parsed, err := strconv.Atoi(sudoGID); err == nil && parsed >= 0 {
			gid = parsed
		}
	}

	return uid, gid
}
