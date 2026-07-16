package bundle

import (
	"fmt"
	"os"
	"path/filepath"

	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"
)

const OCIConfigFileName = "config.json"

func PathForContainer(containerId string, bundle string) (string, error) {
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

func PrepareConfig(containerId string, bundle string) error {
	if bundle == "" {
		return nil
	}

	bundlePath, err := PathForContainer(containerId, bundle)
	if err != nil {
		return err
	}

	configPath := filepath.Join(bundlePath, OCIConfigFileName)
	containerSpec, err := spec.LoadConfigFile(configPath)
	if err != nil {
		return fmt.Errorf("load bundle config: %w", err)
	}
	if err := spec.ValidateBasic(containerSpec); err != nil {
		return fmt.Errorf("validate bundle config: %w", err)
	}
	containerSpec.Root.Path = ResolveRootPath(bundlePath, containerSpec.Root.Path)

	if err := os.MkdirAll(filepath.Join(utils.ContainerDir(containerId), "logs"), 0755); err != nil {
		return fmt.Errorf("create runtime bundle dir: %w", err)
	}
	if err := utils.WriteJsonToFile(utils.ConfigFilePath(containerId), containerSpec); err != nil {
		return fmt.Errorf("write runtime config: %w", err)
	}
	return nil
}

func ResolveRootPath(bundlePath string, rootPath string) string {
	if rootPath == "" || filepath.IsAbs(rootPath) {
		return rootPath
	}
	return filepath.Join(bundlePath, rootPath)
}

func WriteContainerPidFile(pidFile string, pid int) error {
	if pidFile == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(pidFile), 0755); err != nil {
		return err
	}
	return os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644)
}

func WriteExternalPidFileMarker(containerId string, pidFile string) error {
	if pidFile == "" {
		return nil
	}
	return os.WriteFile(utils.ExternalPidFileMarkerPath(containerId), []byte(pidFile), 0644)
}
