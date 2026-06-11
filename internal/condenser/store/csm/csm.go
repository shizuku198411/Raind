package csm

import (
	"fmt"
	"time"
)

func NewCsmManager(csmStore *CsmStore) *CsmManager {
	return &CsmManager{
		csmStore: csmStore,
	}
}

type CsmManager struct {
	csmStore *CsmStore
}

func (m *CsmManager) StoreContainer(containerId string, state string, pid int, tty bool, repo, ref string, command []string, name string, bottleId string, logPath string, podId string) error {
	return m.csmStore.withLock(func(st *ContainerState) error {
		st.Containers[containerId] = ContainerInfo{
			ContainerId:   containerId,
			ContainerName: name,
			PodId:         podId,
			State:         state,
			Pid:           pid,
			ExitCode:      0,
			LogPath:       logPath,
			Tty:           tty,
			Repository:    repo,
			Reference:     ref,
			BottleId:      bottleId,
			Command:       command,
			CreatedAt:     time.Now(),
		}
		return nil
	})
}

func (m *CsmManager) RemoveContainer(containerId string) error {
	return m.csmStore.withLock(func(st *ContainerState) error {
		for id, c := range st.Containers {
			if c.ContainerId == containerId {
				delete(st.Containers, id)
				return nil
			}
		}
		return fmt.Errorf("containerId=%s not found", containerId)
	})
}

func (m *CsmManager) UpdateContainer(containerId string, state string, pid int) error {
	return m.csmStore.withLock(func(st *ContainerState) error {
		c, ok := st.Containers[containerId]
		if !ok {
			return fmt.Errorf("containerId=%s not found", containerId)
		}

		c.State = state
		switch state {
		case "creating":
			if c.CreatedAt.After(c.StoppedAt) {
				c.CreatingAt = time.Now()
			}
		case "created":
			if c.CreatedAt.After(c.StoppedAt) {
				c.CreatedAt = time.Now()
			}
		case "running":
			c.StartedAt = time.Now()
		case "stopped":
			c.StoppedAt = time.Now()
			c.FinishedAt = c.StoppedAt
		}

		if pid >= 0 {
			c.Pid = pid
		}
		st.Containers[containerId] = c
		return nil
	})
}

func (m *CsmManager) UpdateExitStatus(containerId string, exitCode int, reason string, message string) error {
	return m.csmStore.withLock(func(st *ContainerState) error {
		c, ok := st.Containers[containerId]
		if !ok {
			return fmt.Errorf("containerId=%s not found", containerId)
		}

		c.ExitCode = exitCode
		c.Reason = reason
		c.Message = message

		st.Containers[containerId] = c
		return nil
	})
}

func (m *CsmManager) UpdateSpiffe(containerId string, spiffe string) error {
	return m.csmStore.withLock(func(st *ContainerState) error {
		c, ok := st.Containers[containerId]
		if !ok {
			return fmt.Errorf("containerId=%s not found", containerId)
		}
		c.SpiffeId = spiffe
		st.Containers[containerId] = c
		return nil
	})
}

func (m *CsmManager) GetContainerList() ([]ContainerInfo, error) {
	var containerList []ContainerInfo
	err := m.csmStore.withRLock(func(st *ContainerState) error {
		for _, c := range st.Containers {
			containerList = append(containerList, c)
		}
		return nil
	})
	return containerList, err
}

func (m *CsmManager) GetContainersByPodId(podId string) ([]ContainerInfo, error) {
	var containerList []ContainerInfo
	err := m.csmStore.withRLock(func(st *ContainerState) error {
		for _, c := range st.Containers {
			if c.PodId != podId {
				continue
			}
			containerList = append(containerList, c)
		}
		return nil
	})
	return containerList, err
}

func (m *CsmManager) GetContainerById(containerId string) (ContainerInfo, error) {
	var containerInfo ContainerInfo
	err := m.csmStore.withRLock(func(st *ContainerState) error {
		for _, c := range st.Containers {
			if c.ContainerId != containerId {
				continue
			}
			containerInfo = c
			return nil
		}
		return fmt.Errorf("container: %s not found", containerId)
	})
	return containerInfo, err
}

func (m *CsmManager) IsNameAlreadyUsed(name string) bool {
	var result bool
	_ = m.csmStore.withRLock(func(st *ContainerState) error {
		for _, c := range st.Containers {
			if c.ContainerName != name {
				continue
			}
			result = true
			return nil
		}
		result = false
		return nil
	})
	return result
}

func (m *CsmManager) GetContainerIdByName(name string) (string, error) {
	var containerId string
	err := m.csmStore.withRLock(func(st *ContainerState) error {
		for _, c := range st.Containers {
			if c.ContainerName != name {
				continue
			}
			containerId = c.ContainerId
			return nil
		}
		return fmt.Errorf("container: %s not found", name)
	})
	return containerId, err
}

func (m *CsmManager) GetContainerNameById(containerId string) (string, error) {
	var containerName string
	err := m.csmStore.withRLock(func(st *ContainerState) error {
		for _, c := range st.Containers {
			if c.ContainerId != containerId {
				continue
			}
			containerName = c.ContainerName
			return nil
		}
		return fmt.Errorf("container: %s not found", containerId)
	})
	return containerName, err
}

func (m *CsmManager) GetSpiffeById(containerId string) (string, error) {
	var spiffe string
	err := m.csmStore.withRLock(func(st *ContainerState) error {
		for _, c := range st.Containers {
			if c.ContainerId != containerId {
				continue
			}
			spiffe = c.SpiffeId
			return nil
		}
		return fmt.Errorf("container: %s not found", containerId)
	})
	return spiffe, err
}

func (m *CsmManager) GetContainerIdAndName(str string) (id, name string, err error) {
	containerId, getNameErr := m.GetContainerIdByName(str)
	containerName, getIdErr := m.GetContainerNameById(str)
	if getNameErr == nil {
		return containerId, str, nil
	}
	if getIdErr == nil {
		return str, containerName, nil
	}
	return "", "", fmt.Errorf("container: %s not found", str)
}

func (m *CsmManager) ResolveContainerId(str string) (string, error) {
	var containerId string
	// 1. resolve container id by name
	containerId, err := m.GetContainerIdByName(str)
	if err != nil { // the string is not containerId
		// 2. check container exist by id
		if _, err := m.GetContainerById(str); err != nil {
			// the string is not a exist container
			return "", err
		}
		containerId = str
	}
	return containerId, nil
}

func (m *CsmManager) IsContainerExist(str string) bool {
	_, getNameErr := m.GetContainerIdByName(str)
	_, getIdErr := m.GetContainerNameById(str)
	if getNameErr != nil && getIdErr != nil {
		return false
	}
	return true
}

func (m *CsmManager) GetLogPath(containerId string) (string, error) {
	var logPath string
	err := m.csmStore.withRLock(func(st *ContainerState) error {
		for _, c := range st.Containers {
			if c.ContainerId != containerId {
				continue
			}
			logPath = c.LogPath
			return nil
		}
		return fmt.Errorf("container: %s not found", containerId)
	})
	return logPath, err
}
