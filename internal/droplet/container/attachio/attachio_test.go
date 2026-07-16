package attachio

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFramesAndApplyWritesDataFrame(t *testing.T) {
	// == setup ==
	ptmx, err := os.CreateTemp("", "raind-attachio-ptmx-*")
	require.NoError(t, err)
	defer ptmx.Close()
	var input bytes.Buffer
	input.Write(buildFrameForTest(FrameData, []byte("hello")))

	// == exercise ==
	err = ReadFramesAndApply(&input, ptmx)

	// == assert ==
	require.ErrorIs(t, err, io.EOF)
	data, err := os.ReadFile(ptmx.Name())
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestReadFramesAndApplyRejectsTooLargeFrame(t *testing.T) {
	// == setup ==
	ptmx, err := os.CreateTemp("", "raind-attachio-ptmx-*")
	require.NoError(t, err)
	defer ptmx.Close()
	input := bytes.NewBuffer(buildTooLargeFrameForTest(FrameData))

	// == exercise ==
	err = ReadFramesAndApply(input, ptmx)

	// == assert ==
	require.Error(t, err)
	assert.Contains(t, err.Error(), "frame too large")
}

func TestHubAttachReplacesPreviousConnection(t *testing.T) {
	// == setup ==
	ptmx, err := os.CreateTemp("", "raind-attachio-ptmx-*")
	require.NoError(t, err)
	defer ptmx.Close()
	h := NewHub(ptmx, nil, nil)
	oldConnA, oldConnB := netPipeForTest(t)
	defer oldConnB.Close()
	newConnA, newConnB := netPipeForTest(t)
	defer newConnA.Close()
	defer newConnB.Close()

	// == exercise ==
	h.Attach(oldConnA)
	h.Attach(newConnA)

	// == assert ==
	assert.Equal(t, newConnA, h.conn)
	_ = oldConnB.SetWriteDeadline(time.Now().Add(10 * time.Millisecond))
	_, err = oldConnB.Write([]byte("x"))
	require.Error(t, err)
}

func TestWriteFrameWritesTypeLengthAndPayload(t *testing.T) {
	// == setup ==
	var out bytes.Buffer
	payload := []byte("hello")

	// == exercise ==
	err := WriteFrame(&out, FrameData, payload)

	// == assert ==
	require.NoError(t, err)
	written := out.Bytes()
	require.Len(t, written, 10)
	assert.Equal(t, byte(FrameData), written[0])
	assert.Equal(t, uint32(len(payload)), binary.BigEndian.Uint32(written[1:5]))
	assert.Equal(t, payload, written[5:])
}

func TestWriteFrameAllowsEmptyPayload(t *testing.T) {
	// == setup ==
	var out bytes.Buffer

	// == exercise ==
	err := WriteFrame(&out, FrameResize, nil)

	// == assert ==
	require.NoError(t, err)
	written := out.Bytes()
	require.Len(t, written, 5)
	assert.Equal(t, byte(FrameResize), written[0])
	assert.Equal(t, uint32(0), binary.BigEndian.Uint32(written[1:5]))
}

func buildFrameForTest(typ byte, payload []byte) []byte {
	header := make([]byte, 5)
	header[0] = typ
	binary.BigEndian.PutUint32(header[1:5], uint32(len(payload)))
	return append(header, payload...)
}

func buildTooLargeFrameForTest(typ byte) []byte {
	header := make([]byte, 5)
	header[0] = typ
	binary.BigEndian.PutUint32(header[1:5], 8*1024*1024+1)
	return header
}

func netPipeForTest(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	return net.Pipe()
}
