package utils

import (
	"os"
	"path/filepath"
)

func DropletBinPath() string {
	if v := os.Getenv("RAIND_DROPLET_BIN"); v != "" {
		return v
	}
	if snap := os.Getenv("SNAP"); snap != "" {
		return filepath.Join(snap, "bin", "droplet")
	}
	return "/usr/local/bin/droplet"
}

func HookAgentBinPath() string {
	if v := os.Getenv("RAIND_HOOK_AGENT_BIN"); v != "" {
		return v
	}
	if snap := os.Getenv("SNAP"); snap != "" {
		return filepath.Join(snap, "bin", "condenser-hook-agent")
	}
	return "/usr/local/bin/condenser-hook-agent"
}
