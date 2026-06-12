package container

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrintContainerListPrintsHeaderWhenEmpty(t *testing.T) {
	out := captureStdout(t, func() {
		(&ServiceContainerList{}).printContainerList(nil)
	})

	assert.Contains(t, out, "CONTAINER ID")
}

func TestPrintContainerListHandlesRepositoryWithoutSlash(t *testing.T) {
	out := captureStdout(t, func() {
		(&ServiceContainerList{}).printContainerList([]ContainerStateModel{{
			ContainerId: "cid",
			Repository:  "custom-image",
			Reference:   "v1",
			Command:     []string{"/bin/sh"},
			CreatedAt:   time.Now(),
			State:       "running",
			Name:        "web",
		}})
	})

	assert.Contains(t, out, "custom-image:v1")
}

func TestFormatContainerImageNormalizesLibraryRepository(t *testing.T) {
	assert.Equal(t, "alpine:latest", formatContainerImage("library/alpine", "latest"))
	assert.Equal(t, "example.com/ns/app:v1", formatContainerImage("example.com/ns/app", "v1"))
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
