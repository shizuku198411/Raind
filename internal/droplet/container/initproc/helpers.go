package initproc

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
	"raind/internal/droplet/container/namespace"
	"raind/internal/droplet/container/pathenv"
	"raind/internal/droplet/container/rootless"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
	"raind/internal/droplet/utils"
)

func CloseAllExcept012() {
	ents, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return
	}
	for _, e := range ents {
		fd, err := strconv.Atoi(e.Name())
		if err != nil || fd < 3 {
			continue
		}
		_ = syscall.Close(fd)
	}
}

func LookupEntrypointPath(arg0 string, env []string) (string, error) {
	if strings.Contains(arg0, "/") {
		return arg0, nil
	}

	pathVal := pathenv.OrDefault(env)
	for _, dir := range strings.Split(pathVal, ":") {
		if dir == "" {
			dir = "."
		}
		cand := filepath.Join(dir, arg0)
		if err := unix.Access(cand, unix.X_OK); err == nil {
			return cand, nil
		}
	}
	return "", fmt.Errorf("%s: not found in PATH", arg0)
}

func ShouldSkipHostname(containerSpec spec.Spec, prejoinedNamespaces bool) bool {
	nsConfig := namespace.BuildConfig(containerSpec)
	return nsConfig.UTSPath() != "" || (prejoinedNamespaces && rootless.IsSpec(containerSpec))
}

func IsOCIBundleMode(containerId string) bool {
	var state status.StatusObject
	if err := utils.ReadJsonFile(utils.ContainerStatePath(containerId), &state); err != nil {
		return false
	}
	return state.Bundle != "" && filepath.Clean(state.Bundle) != filepath.Clean(utils.ContainerDir(containerId))
}

func DeviceMode(device spec.DeviceObject) (uint32, error) {
	perm := uint32(0666)
	if device.FileMode != nil {
		perm = *device.FileMode
	}
	switch device.Type {
	case "c", "u":
		return syscall.S_IFCHR | perm, nil
	case "b":
		return syscall.S_IFBLK | perm, nil
	case "p":
		return syscall.S_IFIFO | perm, nil
	default:
		return 0, fmt.Errorf("unsupported linux device type: %s", device.Type)
	}
}

func ShouldSkipMissingMaskedPath(path string) bool {
	return strings.HasPrefix(path, "/proc/") || strings.HasPrefix(path, "/sys/")
}

func RlimitResource(name string) (int, bool) {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "RLIMIT_AS":
		return unix.RLIMIT_AS, true
	case "RLIMIT_CORE":
		return unix.RLIMIT_CORE, true
	case "RLIMIT_CPU":
		return unix.RLIMIT_CPU, true
	case "RLIMIT_DATA":
		return unix.RLIMIT_DATA, true
	case "RLIMIT_FSIZE":
		return unix.RLIMIT_FSIZE, true
	case "RLIMIT_LOCKS":
		return unix.RLIMIT_LOCKS, true
	case "RLIMIT_MEMLOCK":
		return unix.RLIMIT_MEMLOCK, true
	case "RLIMIT_MSGQUEUE":
		return unix.RLIMIT_MSGQUEUE, true
	case "RLIMIT_NICE":
		return unix.RLIMIT_NICE, true
	case "RLIMIT_NOFILE":
		return unix.RLIMIT_NOFILE, true
	case "RLIMIT_NPROC":
		return unix.RLIMIT_NPROC, true
	case "RLIMIT_RSS":
		return unix.RLIMIT_RSS, true
	case "RLIMIT_RTPRIO":
		return unix.RLIMIT_RTPRIO, true
	case "RLIMIT_RTTIME":
		return unix.RLIMIT_RTTIME, true
	case "RLIMIT_SIGPENDING":
		return unix.RLIMIT_SIGPENDING, true
	case "RLIMIT_STACK":
		return unix.RLIMIT_STACK, true
	default:
		return 0, false
	}
}
