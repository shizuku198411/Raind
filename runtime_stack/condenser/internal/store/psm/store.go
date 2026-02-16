package psm

import (
	"condenser/internal/utils"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

func NewPsmStore(path string) *PsmStore {
	return &PsmStore{
		path:              path,
		filesystemHandler: utils.NewFilesystemExecutor(),
	}
}

type PsmStore struct {
	path              string
	mu                sync.Mutex
	filesystemHandler utils.FilesystemHandler
}

func (s *PsmStore) withLock(fn func(st *PodState) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lockPath := s.path + ".lock"
	if err := s.filesystemHandler.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	lf, err := s.filesystemHandler.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lf.Close()

	if err := s.filesystemHandler.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer s.filesystemHandler.Flock(int(lf.Fd()), syscall.LOCK_UN)

	st, err := s.loadOrInit()
	if err != nil {
		return err
	}

	if err := fn(st); err != nil {
		return err
	}

	return s.atomicSave(st)
}

func (s *PsmStore) withRLock(fn func(st *PodState) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lockPath := s.path + ".lock"
	if err := s.filesystemHandler.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	lf, err := s.filesystemHandler.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lf.Close()

	if err := s.filesystemHandler.Flock(int(lf.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer s.filesystemHandler.Flock(int(lf.Fd()), syscall.LOCK_UN)

	st, err := s.loadOrInit()
	if err != nil {
		return err
	}

	if err := fn(st); err != nil {
		return err
	}

	return nil
}

func (s *PsmStore) loadOrInit() (*PodState, error) {
	b, err := s.filesystemHandler.ReadFile(s.path)
	if err != nil {
		if s.filesystemHandler.IsNotExist(err) {
			return &PodState{
				Version:      "0.3.0",
				Pods:         map[string]PodInfo{},
				PodTemplates: map[string]PodTemplateInfo{},
				ReplicaSets:  map[string]ReplicaSetInfo{},
			}, nil
		}
		return nil, err
	}

	var st PodState
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, fmt.Errorf("pod state json broken: %w", err)
	}
	if st.Pods == nil {
		st.Pods = map[string]PodInfo{}
	}
	if st.PodTemplates == nil {
		st.PodTemplates = map[string]PodTemplateInfo{}
	}
	if st.ReplicaSets == nil {
		st.ReplicaSets = map[string]ReplicaSetInfo{}
	}
	return &st, nil
}

func (s *PsmStore) atomicSave(st *PodState) error {
	tmp := s.path + ".tmp"

	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')

	f, err := s.filesystemHandler.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return s.filesystemHandler.Rename(tmp, s.path)
}

func (s *PsmStore) SetPodState() error {
	return s.withLock(func(st *PodState) error {
		st.Version = "0.3.0"
		if st.Pods == nil {
			st.Pods = map[string]PodInfo{}
		}
		if st.PodTemplates == nil {
			st.PodTemplates = map[string]PodTemplateInfo{}
		}
		if st.ReplicaSets == nil {
			st.ReplicaSets = map[string]ReplicaSetInfo{}
		}
		return nil
	})
}
