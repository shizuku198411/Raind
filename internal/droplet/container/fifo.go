package container

import (
	"os"
	"syscall"
)

// fifoCreator creates a FIFO (named pipe) at the given path.
//
// This interface represents the lifecycle responsibility for creating
// the FIFO used to synchronize the container runtime and the init process.
type fifoCreator interface {
	createFifo(path string) error
}

// fifoRemover removes an existing FIFO at the given path.
//
// This responsibility is typically used after the synchronization
// process has completed and the FIFO is no longer required.
type fifoRemover interface {
	removeFifo(path string) error
}

// fifoReader waits on a FIFO by opening it for reading and consuming
// a single byte from it.
//
// This is used on the init process side to block until the runtime
// sends a start signal via the FIFO.
type fifoReader interface {
	readFifo(path string) error
}

// fifoWriter signals the init process by writing a byte to the FIFO
// at the given path.
//
// This is used on the runtime side during the container start phase.
type fifoWriter interface {
	writeFifo(path string) error
}

// newContainerFifoHandler returns a containerFifoHandler, which provides
// the concrete implementation of all FIFO-related operations.
//
// The handler groups OS-level FIFO operations into a single struct,
// while allowing callers to depend only on the subset of behavior they
// require via small, focused interfaces.
func newContainerFifoHandler() *containerFifoHandler {
	return &containerFifoHandler{}
}

// containerFifoHandler implements the FIFO creation, deletion,
// read-wait, and write-signal operations using OS-level primitives.
//
// It is the default implementation used by the runtime. Individual
// components (create / init / start) depend only on the minimal
// interface required for their role.
type containerFifoHandler struct{}

// createFifo creates a named pipe (FIFO) at the specified path.
//
// The FIFO is created with mode 0600 to limit access to the owner.
func (c *containerFifoHandler) createFifo(path string) error {
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		return err
	}

	return nil
}

// removeFifo removes the FIFO at the given path.
//
// An error is returned if the file cannot be removed.
func (c *containerFifoHandler) removeFifo(path string) error {
	if err := os.Remove(path); err != nil {
		return err
	}

	return nil
}

// readFifo waits on the FIFO at the given path.
//
// The method opens the FIFO in read-only mode and blocks until a byte is
// written by the writer side. After reading a single byte, the call returns.
//
// This behavior is used as a synchronization barrier for the init process.
func (c *containerFifoHandler) readFifo(path string) error {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	buf := make([]byte, 1)
	if _, err := f.Read(buf); err != nil {
		return err
	}

	return nil
}

// writeFifo sends a synchronization signal by writing a single byte
// to the FIFO at the given path.
//
// This unblocks the reader side, which is waiting in readFifo.
func (c *containerFifoHandler) writeFifo(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write([]byte{1}); err != nil {
		return err
	}

	return nil
}
