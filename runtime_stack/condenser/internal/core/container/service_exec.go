package container

import (
	"condenser/internal/runtime"
	"fmt"
)

// == service: exec container ==
func (s *ContainerService) Exec(execParameter ServiceExecModel) error {
	// resolve container id
	containerId, err := s.csmHandler.ResolveContainerId(execParameter.ContainerId)
	if err != nil {
		return fmt.Errorf("container: %s not found", execParameter.ContainerId)
	}

	// runtime: exec
	if err := s.runtimeHandler.Exec(
		runtime.ExecModel{
			ContainerId: containerId,
			Tty:         execParameter.Tty,
			Entrypoint:  execParameter.Entrypoint,
		},
	); err != nil {
		return err
	}
	return nil
}
