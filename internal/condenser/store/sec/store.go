package sec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"raind/internal/condenser/utils"
	"sync"
	"syscall"
)

func NewSecStore(path string) *SecStore {
	return &SecStore{
		path:              path,
		filesystemHandler: utils.NewFilesystemExecutor(),
	}
}

type SecStore struct {
	path              string
	mu                sync.Mutex
	filesystemHandler utils.FilesystemHandler
}

func (s *SecStore) withLock(fn func(st *SecretState) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lockPath := s.path + ".lock"
	if err := s.ensurePrivateDir(); err != nil {
		return err
	}
	lf, err := s.filesystemHandler.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lf.Close()
	if err := s.filesystemHandler.Chmod(lockPath, 0o600); err != nil {
		return err
	}
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

func (s *SecStore) withRLock(fn func(st *SecretState) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lockPath := s.path + ".lock"
	if err := s.ensurePrivateDir(); err != nil {
		return err
	}
	lf, err := s.filesystemHandler.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lf.Close()
	if err := s.filesystemHandler.Chmod(lockPath, 0o600); err != nil {
		return err
	}
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

func (s *SecStore) ensurePrivateDir() error {
	dir := filepath.Dir(s.path)
	if err := s.filesystemHandler.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return s.filesystemHandler.Chmod(dir, 0o700)
}

func (s *SecStore) loadOrInit() (*SecretState, error) {
	b, err := s.filesystemHandler.ReadFile(s.path)
	if err != nil {
		if s.filesystemHandler.IsNotExist(err) {
			return &SecretState{Version: "0.1.0", Secrets: map[string]SecretInfo{}}, nil
		}
		return nil, err
	}
	var st SecretState
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, fmt.Errorf("secret state json broken: %w", err)
	}
	if st.Secrets == nil {
		st.Secrets = map[string]SecretInfo{}
	}
	return &st, nil
}

func (s *SecStore) atomicSave(st *SecretState) error {
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
	if err := s.filesystemHandler.Chmod(tmp, 0o600); err != nil {
		f.Close()
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
	if err := s.filesystemHandler.Rename(tmp, s.path); err != nil {
		return err
	}
	return s.filesystemHandler.Chmod(s.path, 0o600)
}

func (s *SecStore) SetSecretState() error {
	return s.withLock(func(st *SecretState) error {
		st.Version = "0.1.0"
		if st.Secrets == nil {
			st.Secrets = map[string]SecretInfo{}
		}
		return nil
	})
}
