package container

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerExecShimReadFramesAndApplyWritesDataFrame(t *testing.T) {
	// == setup ==
	execShim := &ContainerExecShim{}
	ptmx, err := os.CreateTemp("", "raind-exec-shim-ptmx-*")
	require.NoError(t, err)
	defer ptmx.Close()
	var input bytes.Buffer
	input.Write(buildFrameForTest(frameData, []byte("hello-exec")))

	// == exercise ==
	err = execShim.readFramesAndApply(&input, ptmx)

	// == assert ==
	require.ErrorIs(t, err, io.EOF)
	data, err := os.ReadFile(ptmx.Name())
	require.NoError(t, err)
	assert.Equal(t, "hello-exec", string(data))
}

func TestContainerExecShimReadFramesAndApplyRejectsTooLargeFrame(t *testing.T) {
	// == setup ==
	execShim := &ContainerExecShim{}
	ptmx, err := os.CreateTemp("", "raind-exec-shim-ptmx-*")
	require.NoError(t, err)
	defer ptmx.Close()
	input := bytes.NewBuffer(buildTooLargeFrameForTest(frameData))

	// == exercise ==
	err = execShim.readFramesAndApply(input, ptmx)

	// == assert ==
	require.Error(t, err)
	assert.Contains(t, err.Error(), "frame too large")
}

func TestExecHubAttachReplacesPreviousConnection(t *testing.T) {
	// == setup ==
	ptmx, err := os.CreateTemp("", "raind-exec-shim-ptmx-*")
	require.NoError(t, err)
	defer ptmx.Close()
	h := newExecHub(ptmx, nil, nil)
	oldConnA, oldConnB := netPipeForTest(t)
	defer oldConnB.Close()
	newConnA, newConnB := netPipeForTest(t)
	defer newConnA.Close()
	defer newConnB.Close()

	// == exercise ==
	h.attach(oldConnA)
	h.attach(newConnA)

	// == assert ==
	assert.Equal(t, newConnA, h.conn)
	_ = oldConnB.SetWriteDeadline(time.Now().Add(10 * time.Millisecond))
	_, err = oldConnB.Write([]byte("x"))
	require.Error(t, err)
}
