package utils

import (
	"fmt"
	"runtime"
)

func HostOs() string {
	return runtime.GOOS
}

func HostArch() (string, error) {
	goarch := runtime.GOARCH
	switch goarch {
	case "amd64", "arm64", "386", "riscv64", "ppc64le", "s390x":
		return goarch, nil
	default:
		return "", fmt.Errorf("unsupported arch: %s", goarch)
	}
}
