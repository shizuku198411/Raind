package container

import (
	"fmt"
	"raind/internal/condenser/utils"
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

	logPath, err := s.csmHandler.GetLogPath(containerId)
	if err != nil {
		return nil, err
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
