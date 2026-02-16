package utils

import (
	"os"
	"path/filepath"
)

const (
	AuditLog      = "/etc/raind/log/droplet_audit.log"
	cgroupRootDir = "/sys/fs/cgroup/raind"
)

func DefaultRootDir() string {
	if v := os.Getenv("RAIND_ROOT_DIR"); v != "" {
		return v
	}
	return "/etc/raind/container"
}

// directory for each container
//
//	e.g. /etc/raind/container/<container-id>
func ContainerDir(containerId string) string {
	return filepath.Join(DefaultRootDir(), containerId)
}

// config.json path
//
//	e.g. /etc/raind/container/<container-id>/config.json
func ConfigFilePath(containerId string) string {
	return filepath.Join(ContainerDir(containerId), "config.json")
}

func ConfigFileHashPath(containerId string) string {
	return filepath.Join(ContainerDir(containerId), "config_hash.json")
}

// state path
func ContainerStatePath(containerId string) string {
	return filepath.Join(ContainerDir(containerId), "state.json")
}

// fifo path
//
//	e.g. /etc/raind/container/<container-id>/exec.fifo
func FifoPath(containerId string) string {
	return filepath.Join(ContainerDir(containerId), "exec.fifo")
}

func SockPath(containerId string) string {
	return filepath.Join(ContainerDir(containerId), "tty.sock")
}

func ExecSockPath(containerId string) string {
	return filepath.Join(ContainerDir(containerId), "exec_tty.sock")
}

// InitPidFilePath returns pidfile path under container dir.
func InitPidFilePath(containerId string) string {
	return filepath.Join(ContainerDir(containerId), "init.pid")
}

// cgroup path
func CgroupPath(containerId string) string {
	return filepath.Join(cgroupRootDir, containerId)
}

// logs
func ShimLogPath(containerId string) string {
	return filepath.Join(ContainerDir(containerId), "logs", "shim.log")
}

func ExecShimLogPath(containerId string) string {
	return filepath.Join(ContainerDir(containerId), "logs", "exec_shim.log")
}

func ConsoleLogPath(containerId string) string {
	return filepath.Join(ContainerDir(containerId), "logs", "console.log")
}

func ExecConsoleLogPath(containerId string) string {
	return filepath.Join(ContainerDir(containerId), "logs", "exec_console.log")
}

func InitLogPath(containerId string) string {
	return filepath.Join(ContainerDir(containerId), "logs", "init.log")
}

func ExecLogPath(containerId string) string {
	return filepath.Join(ContainerDir(containerId), "logs", "exec.log")
}
