package container

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"raind/internal/condenser/utils"
)

// == service: container config spec ==
func (s *ContainerService) GetContainerSpec(target string) (map[string]any, error) {
	containerId, err := s.csmHandler.ResolveContainerId(target)
	if err != nil {
		return nil, fmt.Errorf("container: %s not found", target)
	}
	if !s.csmHandler.IsContainerExist(containerId) {
		return nil, fmt.Errorf("container: %s not found", target)
	}

	specPath := filepath.Join(utils.ContainerRootDir, containerId, "config.json")
	raw, err := s.filesystemHandler.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("read container config spec failed: %w", err)
	}

	var spec map[string]any
	if err := json.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("decode container config spec failed: %w", err)
	}
	return spec, nil
}
