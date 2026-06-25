package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const MaxConsoleSize = 65535

func ValidateBasic(containerSpec Spec) error {
	if strings.TrimSpace(containerSpec.OciVersion) == "" {
		return fmt.Errorf("ociVersion is required")
	}
	if containerSpec.OciVersion != "1.3.0" {
		return fmt.Errorf("unsupported ociVersion: %s", containerSpec.OciVersion)
	}
	if strings.TrimSpace(containerSpec.Root.Path) == "" {
		return fmt.Errorf("root.path is required")
	}
	if containerSpec.Process.ConsoleSize != nil {
		if containerSpec.Process.ConsoleSize.Height == 0 || containerSpec.Process.ConsoleSize.Width == 0 {
			return fmt.Errorf("process.consoleSize height and width must be positive")
		}
		if containerSpec.Process.ConsoleSize.Height > MaxConsoleSize || containerSpec.Process.ConsoleSize.Width > MaxConsoleSize {
			return fmt.Errorf("process.consoleSize height and width must be <= %d", MaxConsoleSize)
		}
	}
	if containerSpec.Process.OOMScoreAdj != nil {
		if *containerSpec.Process.OOMScoreAdj < -1000 || *containerSpec.Process.OOMScoreAdj > 1000 {
			return fmt.Errorf("process.oomScoreAdj must be between -1000 and 1000")
		}
	}
	for _, ns := range containerSpec.LinuxSpec.Namespaces {
		switch ns.Type {
		case "mount", "network", "uts", "pid", "ipc", "user", "cgroup":
		default:
			return fmt.Errorf("unsupported linux namespace type: %s", ns.Type)
		}
		if ns.Path != "" {
			if err := validateNamespacePath(ns.Type, ns.Path); err != nil {
				return err
			}
		}
	}
	for _, path := range containerSpec.LinuxSpec.MaskedPaths {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("linux masked path must be absolute: %s", path)
		}
	}
	for _, path := range containerSpec.LinuxSpec.ReadonlyPaths {
		if !filepath.IsAbs(path) {
			return fmt.Errorf("linux readonly path must be absolute: %s", path)
		}
	}
	for _, mapping := range append(containerSpec.LinuxSpec.UIDMappings, containerSpec.LinuxSpec.GIDMappings...) {
		if mapping.Size <= 0 {
			return fmt.Errorf("linux id mapping size must be positive")
		}
		if mapping.ContainerID < 0 || mapping.HostID < 0 {
			return fmt.Errorf("linux id mapping ids must be non-negative")
		}
	}
	for _, device := range containerSpec.LinuxSpec.Devices {
		if strings.TrimSpace(device.Path) == "" {
			return fmt.Errorf("linux device path is required")
		}
		switch device.Type {
		case "c", "u", "b", "p":
		default:
			return fmt.Errorf("unsupported linux device type: %s", device.Type)
		}
	}
	for _, rule := range containerSpec.LinuxSpec.Resources.Devices {
		if rule.Type != "" {
			switch rule.Type {
			case "a", "b", "c":
			default:
				return fmt.Errorf("unsupported linux resources device type: %s", rule.Type)
			}
		}
		if rule.Major != nil && *rule.Major < 0 {
			return fmt.Errorf("linux resources device major must be non-negative")
		}
		if rule.Minor != nil && *rule.Minor < 0 {
			return fmt.Errorf("linux resources device minor must be non-negative")
		}
		for _, access := range rule.Access {
			switch access {
			case 'r', 'w', 'm':
			default:
				return fmt.Errorf("unsupported linux resources device access: %c", access)
			}
		}
	}
	for _, capName := range appendCapabilityNames(containerSpec.Process.Capabilities) {
		if _, ok := supportedCapabilities[capName]; !ok {
			return fmt.Errorf("unsupported process capability: %s", capName)
		}
	}
	for _, rlimit := range containerSpec.Process.Rlimits {
		if _, ok := supportedRlimits[strings.ToUpper(strings.TrimSpace(rlimit.Type))]; !ok {
			return fmt.Errorf("unsupported process rlimit type: %s", rlimit.Type)
		}
	}
	return nil
}

func validateNamespacePath(nsType string, path string) error {
	target, err := os.Readlink(path)
	if err != nil {
		return fmt.Errorf("read namespace path %s: %w", path, err)
	}
	want := namespaceLinkName(nsType)
	if want == "" {
		return fmt.Errorf("unsupported linux namespace type: %s", nsType)
	}
	if !strings.HasPrefix(target, want+":") {
		return fmt.Errorf("namespace path %s has type %q, expected %q", path, target, want)
	}
	return nil
}

func namespaceLinkName(nsType string) string {
	switch nsType {
	case "mount":
		return "mnt"
	case "network":
		return "net"
	default:
		return nsType
	}
}

func appendCapabilityNames(capabilities CapabilityObject) []string {
	out := []string{}
	out = append(out, capabilities.Bounding...)
	out = append(out, capabilities.Permitted...)
	out = append(out, capabilities.Inheritable...)
	out = append(out, capabilities.Effective...)
	out = append(out, capabilities.Ambient...)
	return out
}

var supportedCapabilities = map[string]struct{}{
	"CAP_CHOWN": {}, "CAP_DAC_OVERRIDE": {}, "CAP_DAC_READ_SEARCH": {}, "CAP_FOWNER": {},
	"CAP_FSETID": {}, "CAP_KILL": {}, "CAP_SETGID": {}, "CAP_SETUID": {},
	"CAP_SETPCAP": {}, "CAP_LINUX_IMMUTABLE": {}, "CAP_NET_BIND_SERVICE": {},
	"CAP_NET_BROADCAST": {}, "CAP_NET_ADMIN": {}, "CAP_NET_RAW": {}, "CAP_IPC_LOCK": {},
	"CAP_IPC_OWNER": {}, "CAP_SYS_MODULE": {}, "CAP_SYS_RAWIO": {}, "CAP_SYS_CHROOT": {},
	"CAP_SYS_PTRACE": {}, "CAP_SYS_PACCT": {}, "CAP_SYS_ADMIN": {}, "CAP_SYS_BOOT": {},
	"CAP_SYS_NICE": {}, "CAP_SYS_RESOURCE": {}, "CAP_SYS_TIME": {}, "CAP_SYS_TTY_CONFIG": {},
	"CAP_MKNOD": {}, "CAP_LEASE": {}, "CAP_AUDIT_WRITE": {}, "CAP_AUDIT_CONTROL": {},
	"CAP_SETFCAP": {}, "CAP_MAC_OVERRIDE": {}, "CAP_MAC_ADMIN": {}, "CAP_SYSLOG": {},
	"CAP_WAKE_ALARM": {}, "CAP_BLOCK_SUSPEND": {}, "CAP_AUDIT_READ": {}, "CAP_PERFMON": {},
	"CAP_BPF": {}, "CAP_CHECKPOINT_RESTORE": {},
}

var supportedRlimits = map[string]struct{}{
	"RLIMIT_AS": {}, "RLIMIT_CORE": {}, "RLIMIT_CPU": {}, "RLIMIT_DATA": {},
	"RLIMIT_FSIZE": {}, "RLIMIT_LOCKS": {}, "RLIMIT_MEMLOCK": {}, "RLIMIT_MSGQUEUE": {},
	"RLIMIT_NICE": {}, "RLIMIT_NOFILE": {}, "RLIMIT_NPROC": {}, "RLIMIT_RSS": {},
	"RLIMIT_RTPRIO": {}, "RLIMIT_RTTIME": {}, "RLIMIT_SIGPENDING": {}, "RLIMIT_STACK": {},
}
