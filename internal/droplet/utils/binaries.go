package utils

import "os"

func SelfBinPath() string {
	if v := os.Getenv("RAIND_DROPLET_SELF_BIN"); v != "" {
		return v
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		return exe
	}
	return os.Args[0]
}
