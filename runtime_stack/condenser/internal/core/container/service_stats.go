package container

import (
	"bufio"
	"encoding/json"
	"fmt"
	"time"

	"condenser/internal/utils"
)

// == service: container stats ==
func (s *ContainerService) GetContainerStats(target string) (ContainerStats, error) {
	containerId, err := s.csmHandler.ResolveContainerId(target)
	if err != nil {
		return ContainerStats{}, fmt.Errorf("container: %s not found", target)
	}
	if !s.csmHandler.IsContainerExist(containerId) {
		return ContainerStats{}, fmt.Errorf("container: %s not found", target)
	}
	containerInfo, err := s.csmHandler.GetContainerById(containerId)
	if err != nil {
		return ContainerStats{}, err
	}

	statsMap, err := s.loadLatestStats()
	if err != nil {
		return ContainerStats{}, err
	}
	stat, ok := statsMap[containerId]
	if !ok {
		stat = ContainerStats{ContainerID: containerId}
	}
	stat.Status = containerInfo.State
	stat.ContainerName = containerInfo.ContainerName
	stat.PodId = containerInfo.PodId
	stat.SpiffeID = containerInfo.SpiffeId
	stat.Pid = containerInfo.Pid
	stat.ExitCode = containerInfo.ExitCode
	stat.Reason = containerInfo.Reason
	stat.Message = containerInfo.Message
	stat.LogPath = containerInfo.LogPath
	stat.Repository = containerInfo.Repository
	stat.Reference = containerInfo.Reference
	stat.Command = containerInfo.Command
	stat.BottleId = containerInfo.BottleId
	stat.CreatingAt = containerInfo.CreatingAt
	stat.CreatedAt = containerInfo.CreatedAt
	stat.StartedAt = containerInfo.StartedAt
	stat.StoppedAt = containerInfo.StoppedAt
	stat.FinishedAt = containerInfo.FinishedAt
	stat.Labels = containerInfo.Labels
	stat.Annotations = containerInfo.Annotaions
	stat.Attempt = containerInfo.Attemp
	stat.Tty = containerInfo.Tty
	return stat, nil
}

// == service: list container stats ==
func (s *ContainerService) ListContainerStats() ([]ContainerStats, error) {
	statsMap, err := s.loadLatestStats()
	if err != nil {
		return nil, err
	}
	containerList, err := s.csmHandler.GetContainerList()
	if err != nil {
		return nil, err
	}
	alive := map[string]struct{}{}
	infoByID := map[string]struct {
		name   string
		spiffe string
		pid    int
		state  string
		podId  string
		info   struct {
			exitCode    int
			reason      string
			message     string
			logPath     string
			repository  string
			reference   string
			command     []string
			bottleId    string
			creatingAt  time.Time
			createdAt   time.Time
			startedAt   time.Time
			stoppedAt   time.Time
			finishedAt  time.Time
			labels      map[string]string
			annotations map[string]string
			attempt     uint32
			tty         bool
		}
	}{}
	for _, c := range containerList {
		alive[c.ContainerId] = struct{}{}
		infoByID[c.ContainerId] = struct {
			name   string
			spiffe string
			pid    int
			state  string
			podId  string
			info   struct {
				exitCode    int
				reason      string
				message     string
				logPath     string
				repository  string
				reference   string
				command     []string
				bottleId    string
				creatingAt  time.Time
				createdAt   time.Time
				startedAt   time.Time
				stoppedAt   time.Time
				finishedAt  time.Time
				labels      map[string]string
				annotations map[string]string
				attempt     uint32
				tty         bool
			}
		}{
			name:   c.ContainerName,
			spiffe: c.SpiffeId,
			pid:    c.Pid,
			state:  c.State,
			podId:  c.PodId,
			info: struct {
				exitCode    int
				reason      string
				message     string
				logPath     string
				repository  string
				reference   string
				command     []string
				bottleId    string
				creatingAt  time.Time
				createdAt   time.Time
				startedAt   time.Time
				stoppedAt   time.Time
				finishedAt  time.Time
				labels      map[string]string
				annotations map[string]string
				attempt     uint32
				tty         bool
			}{
				exitCode:    c.ExitCode,
				reason:      c.Reason,
				message:     c.Message,
				logPath:     c.LogPath,
				repository:  c.Repository,
				reference:   c.Reference,
				command:     c.Command,
				bottleId:    c.BottleId,
				creatingAt:  c.CreatingAt,
				createdAt:   c.CreatedAt,
				startedAt:   c.StartedAt,
				stoppedAt:   c.StoppedAt,
				finishedAt:  c.FinishedAt,
				labels:      c.Labels,
				annotations: c.Annotaions,
				attempt:     c.Attemp,
				tty:         c.Tty,
			},
		}
	}
	var list []ContainerStats
	for _, stat := range statsMap {
		if _, ok := alive[stat.ContainerID]; !ok {
			continue
		}
		if info, ok := infoByID[stat.ContainerID]; ok {
			stat.ContainerName = info.name
			stat.SpiffeID = info.spiffe
			stat.Pid = info.pid
			stat.Status = info.state
			stat.PodId = info.podId
			stat.ExitCode = info.info.exitCode
			stat.Reason = info.info.reason
			stat.Message = info.info.message
			stat.LogPath = info.info.logPath
			stat.Repository = info.info.repository
			stat.Reference = info.info.reference
			stat.Command = info.info.command
			stat.BottleId = info.info.bottleId
			stat.CreatingAt = info.info.creatingAt
			stat.CreatedAt = info.info.createdAt
			stat.StartedAt = info.info.startedAt
			stat.StoppedAt = info.info.stoppedAt
			stat.FinishedAt = info.info.finishedAt
			stat.Labels = info.info.labels
			stat.Annotations = info.info.annotations
			stat.Attempt = info.info.attempt
			stat.Tty = info.info.tty
		}
		list = append(list, stat)
	}
	return list, nil
}

func (s *ContainerService) loadLatestStats() (map[string]ContainerStats, error) {
	f, err := s.filesystemHandler.Open(utils.MetricsLogPath)
	if err != nil {
		if s.filesystemHandler.IsNotExist(err) {
			return map[string]ContainerStats{}, nil
		}
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// allow large lines just in case
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	statsMap := map[string]ContainerStats{}
	for scanner.Scan() {
		var record ContainerStats
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			continue
		}
		if record.ContainerID == "" {
			continue
		}
		statsMap[record.ContainerID] = record
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return statsMap, nil
}
