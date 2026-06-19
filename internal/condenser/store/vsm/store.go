package vsm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"raind/internal/condenser/utils"
	"sync"
	"syscall"
)

func NewStore(path string) *Store {
	return &Store{
		path:              path,
		filesystemHandler: utils.NewFilesystemExecutor(),
	}
}

type Store struct {
	path              string
	mu                sync.Mutex
	filesystemHandler utils.FilesystemHandler
}

func (s *Store) withLock(fn func(st *VolumeState) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lockPath := s.path + ".lock"
	if err := s.filesystemHandler.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	lf, err := s.filesystemHandler.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
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

func (s *Store) withRLock(fn func(st *VolumeState) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lockPath := s.path + ".lock"
	if err := s.filesystemHandler.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	lf, err := s.filesystemHandler.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
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
	return fn(st)
}

func (s *Store) loadOrInit() (*VolumeState, error) {
	b, err := s.filesystemHandler.ReadFile(s.path)
	if err != nil {
		if s.filesystemHandler.IsNotExist(err) {
			return &VolumeState{Version: "0.1.0", PVCs: map[string]PersistentVolumeClaimInfo{}}, nil
		}
		return nil, err
	}
	var st VolumeState
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, fmt.Errorf("volume state json broken: %w", err)
	}
	if st.PVCs == nil {
		st.PVCs = map[string]PersistentVolumeClaimInfo{}
	}
	return &st, nil
}

func (s *Store) atomicSave(st *VolumeState) error {
	tmp := s.path + ".tmp"
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	f, err := s.filesystemHandler.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
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

func (s *Store) SetVolumeState() error {
	return s.withLock(func(st *VolumeState) error {
		st.Version = "0.1.0"
		if st.PVCs == nil {
			st.PVCs = map[string]PersistentVolumeClaimInfo{}
		}
		return nil
	})
}
