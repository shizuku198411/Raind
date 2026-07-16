package container

import (
	"bytes"
	"encoding/binary"
	"raind/internal/droplet/container/attachio"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContainerAttachWriteFrameWritesTypeLengthAndPayload(t *testing.T) {
	// == setup ==
	var out bytes.Buffer
	attach := &ContainerAttach{}
	payload := []byte("hello")

	// == exercise ==
	err := attach.writeFrame(&out, attachio.FrameData, payload)

	// == assert ==
	require.NoError(t, err)
	written := out.Bytes()
	require.Len(t, written, 10)
	assert.Equal(t, byte(attachio.FrameData), written[0])
	assert.Equal(t, uint32(len(payload)), binary.BigEndian.Uint32(written[1:5]))
	assert.Equal(t, payload, written[5:])
}

func TestContainerAttachWriteFrameAllowsEmptyPayload(t *testing.T) {
	// == setup ==
	var out bytes.Buffer
	attach := &ContainerAttach{}

	// == exercise ==
	err := attach.writeFrame(&out, attachio.FrameResize, nil)

	// == assert ==
	require.NoError(t, err)
	written := out.Bytes()
	require.Len(t, written, 5)
	assert.Equal(t, byte(attachio.FrameResize), written[0])
	assert.Equal(t, uint32(0), binary.BigEndian.Uint32(written[1:5]))
}
