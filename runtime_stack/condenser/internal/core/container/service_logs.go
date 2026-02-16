package container

import (
	"condenser/internal/utils"
	"fmt"
	"path/filepath"
)

const (
	maxTailLines = 5000
	maxTailBytes = 4 * 1024 * 1024
)

func (s *ContainerService) GetLogWithTailLines(target string, n int) ([]byte, error) {
	// resolve container id
	containerId, err := s.csmHandler.ResolveContainerId(target)
	if err != nil {
		return nil, fmt.Errorf("container: %s not found", target)
	}

	// check the container running tty/non tty
	containerInfo, err := s.csmHandler.GetContainerById(containerId)
	if err != nil {
		return nil, err
	}
	var logPath string
	switch containerInfo.Tty {
	case true:
		logPath = filepath.Join(utils.ContainerRootDir, containerId, "logs", "console.log")
	case false:
		logPath = filepath.Join(utils.ContainerRootDir, containerId, "logs", "init.log")
	}

	if n > maxTailLines {
		return nil, fmt.Errorf("invalid tail lines: max=%d", maxTailLines)
	}

	data, err := utils.TailLines(logPath, n, maxTailBytes)
	if err != nil {
		return nil, fmt.Errorf("tail failed: %v", err)
	}

	return data, nil
}
