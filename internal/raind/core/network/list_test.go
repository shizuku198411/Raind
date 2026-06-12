package network

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintNetworkListPrintsHeaderAndRows(t *testing.T) {
	out := captureStdout(t, func() {
		(&ServiceNetworkList{}).printNetworkList([]NetworkInfoModel{{Interface: "raind0", Address: "10.166.0.254/24", NumContainers: 1}})
	})

	assert.Contains(t, out, "NETWORK")
	assert.Contains(t, out, "raind0")
}

func TestPrintNetworkListPrintsHeaderWhenEmpty(t *testing.T) {
	out := captureStdout(t, func() {
		(&ServiceNetworkList{}).printNetworkList(nil)
	})

	assert.Contains(t, out, "NETWORK")
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
