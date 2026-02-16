package ilm

import (
	"condenser/internal/utils"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

func NewIlmStore(path string) *IlmStore {
	return &IlmStore{
		path:              path,
		filesystemHandler: utils.NewFilesystemExecutor(),
	}
}

type IlmStore struct {
	path              string
	mu                sync.Mutex
	filesystemHandler utils.FilesystemHandler
}

func (s *IlmStore) withLock(fn func(st *ImageLayerState) error) error {
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

func (s *IlmStore) withRLock(fn func(st *ImageLayerState) error) error {
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

func (s *IlmStore) loadOrInit() (*ImageLayerState, error) {
	b, err := s.filesystemHandler.ReadFile(s.path)
	if err != nil {
		if s.filesystemHandler.IsNotExist(err) {
			// image layer state file not exist
			return &ImageLayerState{
				Version:      "0.1.0",
				Repositories: map[string]RepositoryInfo{},
			}, nil
		}
		return nil, err
	}

	var st ImageLayerState
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, fmt.Errorf("image layer state json broken: %w", err)
	}
	return &st, nil
}

func (s *IlmStore) atomicSave(st *ImageLayerState) error {
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

func (s *IlmStore) SetConfig() error {
	return s.withLock(func(st *ImageLayerState) error {
		st.Version = "0.1.0"
		if st.Repositories == nil {
			st.Repositories = map[string]RepositoryInfo{}
		}
		return nil
	})
}
