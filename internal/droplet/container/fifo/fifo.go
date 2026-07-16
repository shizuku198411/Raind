package fifo

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const (
	WriteTimeout       = 5 * time.Second
	WriteRetryInterval = 20 * time.Millisecond
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) Create(path string) error {
	return syscall.Mkfifo(path, 0o600)
}

func (h *Handler) Remove(path string) error {
	return os.Remove(path)
}

func (h *Handler) Read(path string) error {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 1)
	_, err = f.Read(buf)
	return err
}

func (h *Handler) Write(path string) error {
	return h.WriteWithTimeout(path, WriteTimeout)
}

func (h *Handler) WriteWithTimeout(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		fd, err := unix.Open(path, unix.O_WRONLY|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
		if err == nil {
			defer unix.Close(fd)

			for {
				n, err := unix.Write(fd, []byte{1})
				if err == nil {
					if n != 1 {
						return fmt.Errorf("fifo write %s: short write: wrote %d bytes", path, n)
					}
					return nil
				}
				if errors.Is(err, unix.EINTR) {
					continue
				}
				return fmt.Errorf("fifo write %s: %w", path, err)
			}
		}

		if !errors.Is(err, unix.ENXIO) && !errors.Is(err, unix.EINTR) {
			return fmt.Errorf("fifo open %s: %w", path, err)
		}

		if timeout <= 0 || time.Now().After(deadline) {
			return fmt.Errorf("fifo open %s: timed out waiting for reader after %s", path, timeout)
		}

		time.Sleep(WriteRetryInterval)
	}
}
