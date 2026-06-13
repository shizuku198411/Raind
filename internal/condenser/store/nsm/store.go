package nsm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"raind/internal/condenser/utils"
	"sync"
	"syscall"
)

func NewNsmStore(path string) *NsmStore {
	return &NsmStore{
		path:              path,
		filesystemHandler: utils.NewFilesystemExecutor(),
	}
}

type NsmStore struct {
	path              string
	mu                sync.Mutex
	filesystemHandler utils.FilesystemHandler
}

func (s *NsmStore) withLock(fn func(st *NamespaceState) error) error {
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

func (s *NsmStore) withRLock(fn func(st *NamespaceState) error) error {
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
	return fn(st)
}

func (s *NsmStore) loadOrInit() (*NamespaceState, error) {
	b, err := s.filesystemHandler.ReadFile(s.path)
	if err != nil {
		if s.filesystemHandler.IsNotExist(err) {
			return defaultNamespaceState(), nil
		}
		return nil, err
	}

	var st NamespaceState
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, fmt.Errorf("namespace state json broken: %w", err)
	}
	if st.Namespace == nil {
		st.Namespace = map[string]NamespaceInfo{}
	}
	if st.Version == "" {
		st.Version = "0.1.0"
	}
	ensureDefaultNamespace(st.Namespace)
	return &st, nil
}

func (s *NsmStore) atomicSave(st *NamespaceState) error {
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

func (s *NsmStore) SetNamespaceState() error {
	return s.withLock(func(st *NamespaceState) error {
		st.Version = "0.1.0"
		if st.Namespace == nil {
			st.Namespace = map[string]NamespaceInfo{}
		}
		ensureDefaultNamespace(st.Namespace)
		return nil
	})
}

func defaultNamespaceState() *NamespaceState {
	ns := map[string]NamespaceInfo{}
	ensureDefaultNamespace(ns)
	return &NamespaceState{
		Version:   "0.1.0",
		Namespace: ns,
	}
}
