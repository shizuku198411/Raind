package bottle

import (
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintBottleListPrintsHeaderWhenEmpty(t *testing.T) {
	out := captureStdout(t, func() {
		(&ServiceBottleList{}).printBottleList(nil)
	})

	assert.Contains(t, out, "BOTTLE ID")
}

func TestFormatImageHandlesLibraryAndPlainRepository(t *testing.T) {
	assert.Equal(t, "alpine:latest", formatImage("library/alpine:latest"))
	assert.Equal(t, "custom-image:v1", formatImage("custom-image:v1"))
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
