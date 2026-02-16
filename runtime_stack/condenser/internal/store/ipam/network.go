package ipam

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

func GetDefaultInterfaceIpv4() (string, error) {
	cmd := exec.Command("ip", "-4", "route", "show", "default")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("run ip route: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return "", fmt.Errorf("no defauilt route found (ipv4)")
	}

	// retrieve device name
	fields := strings.Fields(lines[0])
	for i := 0; i < len(fields)-1; i++ {
		if fields[i] == "dev" {
			return fields[i+1], nil
		}
	}
	return "", fmt.Errorf("cannot find dev in: %q", lines[0])
}

func GetDefaultInterfaceAddressIpv4(interfaceName string) (string, error) {
	cmd := exec.Command("ip", "-4", "addr", "show", interfaceName)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`inet\s+([0-9.]+/\d+)`)
	m := re.FindSubmatch(out)
	if len(m) < 2 {
		return "", fmt.Errorf("no ipv4 address found for %s", interfaceName)
	}
	return string(m[1]), nil
}
