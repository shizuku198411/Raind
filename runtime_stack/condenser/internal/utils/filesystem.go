package utils

import (
	"io"
	"os"
	"syscall"
)

type FilesystemHandler interface {
	MkdirAll(path string, perm os.FileMode) error
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	Open(name string) (*os.File, error)
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	Copy(dst io.Writer, src io.Reader) (int64, error)
	Remove(name string) error
	RemoveAll(path string) error
	Rename(oldpath string, newpath string) error
	IsNotExist(err error) bool
	Flock(fd int, how int) error
	Chmod(name string, mode os.FileMode) error
}

func NewFilesystemExecutor() *FilesystemExecutor {
	return &FilesystemExecutor{}
}

type FilesystemExecutor struct{}

func (e *FilesystemExecutor) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (s *FilesystemExecutor) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

func (s *FilesystemExecutor) WriteFile(name string, data []byte, perm os.FileMode) error {
	return os.WriteFile(name, data, perm)
}

func (s *FilesystemExecutor) Open(name string) (*os.File, error) {
	return os.Open(name)
}

func (s *FilesystemExecutor) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

func (s *FilesystemExecutor) Copy(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}

func (s *FilesystemExecutor) Remove(name string) error {
	return os.Remove(name)
}

func (s *FilesystemExecutor) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (s *FilesystemExecutor) Rename(oldpath string, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func (s *FilesystemExecutor) IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

func (s *FilesystemExecutor) Flock(fd int, how int) error {
	return syscall.Flock(fd, how)
}

func (s *FilesystemExecutor) Chmod(name string, mode os.FileMode) error {
	return os.Chmod(name, mode)
}
