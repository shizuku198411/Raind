package container

import (
	"fmt"
	"os"
	"path/filepath"

	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"
)

const ociConfigFileName = "config.json"

func bundlePathForContainer(containerId string, bundle string) (string, error) {
	if bundle == "" {
		return utils.ContainerDir(containerId), nil
	}

	abs, err := filepath.Abs(bundle)
	if err != nil {
		return "", err
	}
	if info, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("stat bundle: %w", err)
	} else if !info.IsDir() {
		return "", fmt.Errorf("bundle is not a directory: %s", abs)
	}
	return abs, nil
}

func prepareBundleConfig(containerId string, bundle string) error {
	if bundle == "" {
		return nil
	}

	bundlePath, err := bundlePathForContainer(containerId, bundle)
	if err != nil {
		return err
	}

	configPath := filepath.Join(bundlePath, ociConfigFileName)
	containerSpec, err := spec.LoadConfigFile(configPath)
	if err != nil {
		return fmt.Errorf("load bundle config: %w", err)
	}
	if err := spec.ValidateBasic(containerSpec); err != nil {
		return fmt.Errorf("validate bundle config: %w", err)
	}
	containerSpec.Root.Path = resolveBundleRootPath(bundlePath, containerSpec.Root.Path)

	if err := os.MkdirAll(filepath.Join(utils.ContainerDir(containerId), "logs"), 0755); err != nil {
		return fmt.Errorf("create runtime bundle dir: %w", err)
	}
	if err := utils.WriteJsonToFile(utils.ConfigFilePath(containerId), containerSpec); err != nil {
		return fmt.Errorf("write runtime config: %w", err)
	}
	return nil
}

func resolveBundleRootPath(bundlePath string, rootPath string) string {
	if rootPath == "" || filepath.IsAbs(rootPath) {
		return rootPath
	}
	return filepath.Join(bundlePath, rootPath)
}

func writeContainerPidFile(pidFile string, pid int) error {
	if pidFile == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(pidFile), 0755); err != nil {
		return err
	}
	return os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", pid)), 0644)
}

func writeSpecHashFile(containerId string) error {
	hash, err := utils.Sha256File(utils.ConfigFilePath(containerId))
	if err != nil {
		return err
	}
	return utils.WriteJsonToFile(utils.ConfigFileHashPath(containerId), spec.SpecHash{Sha256: hash})
}
