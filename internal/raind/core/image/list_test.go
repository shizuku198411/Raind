package image

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintImageListPrintsHeaderAndFormatsLibraryRepo(t *testing.T) {
	out := captureStdout(t, func() {
		(&ServiceImageList{}).printImageList([]ImageDataModel{{Repository: "library/alpine", Reference: "latest", CreatedAt: time.Now()}})
	})

	assert.Contains(t, out, "REPOSITORY")
	assert.Contains(t, out, "alpine")
	assert.Contains(t, out, "latest")
}

func TestPrintImageListPrintsHeaderWhenEmpty(t *testing.T) {
	out := captureStdout(t, func() {
		(&ServiceImageList{}).printImageList(nil)
	})

	assert.Contains(t, out, "REPOSITORY")
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
