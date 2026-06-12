package service

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintServiceListPrintsHeaderWhenEmpty(t *testing.T) {
	out := captureStdout(t, func() {
		(&ServiceServiceList{}).printServiceList(nil)
	})

	assert.Contains(t, out, "SERVICE ID")
}

func TestFormatPortsDefaultsProtocolAndEmptyValue(t *testing.T) {
	assert.Equal(t, "-", formatPorts(nil))
	assert.Equal(t, "80->8080/tcp", formatPorts([]ServicePortModel{{Port: 80, TargetPort: 8080}}))
	assert.Equal(t, "53->53/udp", formatPorts([]ServicePortModel{{Port: 53, TargetPort: 53, Protocol: "UDP"}}))
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()
	require.NoError(t, w.Close())
	b, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(b)
}
