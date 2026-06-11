package container

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerFifoHandlerCreateAndRemoveFifo(t *testing.T) {
	// == setup ==
	handler := &containerFifoHandler{}
	fifoPath := filepath.Join(t.TempDir(), "exec.fifo")

	// == exercise ==
	err := handler.createFifo(fifoPath)

	// == assert ==
	require.NoError(t, err)
	assert.FileExists(t, fifoPath)

	// == exercise ==
	err = handler.removeFifo(fifoPath)

	// == assert ==
	require.NoError(t, err)
	assert.NoFileExists(t, fifoPath)
}

func TestContainerFifoHandlerReadBlocksUntilWrite(t *testing.T) {
	// == setup ==
	handler := &containerFifoHandler{}
	fifoPath := filepath.Join(t.TempDir(), "exec.fifo")
	require.NoError(t, handler.createFifo(fifoPath))
	errCh := make(chan error, 1)

	// == exercise ==
	go func() {
		errCh <- handler.readFifo(fifoPath)
	}()
	select {
	case err := <-errCh:
		require.NoError(t, err)
		t.Fatal("readFifo returned before writeFifo")
	case <-time.After(20 * time.Millisecond):
	}
	require.NoError(t, handler.writeFifo(fifoPath))

	// == assert ==
	require.NoError(t, <-errCh)
}
