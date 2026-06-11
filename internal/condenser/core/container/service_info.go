package container

import (
	"fmt"
)

// == service: stop ==
func (s *ContainerService) GetContainerLogPath(target string) (string, error) {
	// resolve container id
	containerId, err := s.csmHandler.ResolveContainerId(target)
	if err != nil {
		return "", fmt.Errorf("container: %s not found", target)
	}

	logPath, err := s.csmHandler.GetLogPath(containerId)
	if err != nil {
		return "", err
	}

	return logPath, nil
}
