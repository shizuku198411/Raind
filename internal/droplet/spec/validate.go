package spec

import (
	"fmt"
	"strings"
)

const MaxConsoleSize = 65535

func ValidateBasic(containerSpec Spec) error {
	if strings.TrimSpace(containerSpec.OciVersion) == "" {
		return fmt.Errorf("ociVersion is required")
	}
	if strings.TrimSpace(containerSpec.Root.Path) == "" {
		return fmt.Errorf("root.path is required")
	}
	if len(containerSpec.Process.Args) == 0 || strings.TrimSpace(containerSpec.Process.Args[0]) == "" {
		return fmt.Errorf("process.args[0] is required")
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
	return nil
}
