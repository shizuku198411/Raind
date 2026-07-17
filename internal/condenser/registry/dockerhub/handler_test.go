package dockerhub

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJoinRootReturnsAbsolutePathInsideRoot(t *testing.T) {
	rootfs := t.TempDir()
	registry := &RegistryDockerHub{}

	got, err := registry.joinRoot(rootfs, "etc/passwd")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(rootfs, "etc", "passwd"), got)
}

func TestJoinRootDeanchorsAbsoluteArchivePath(t *testing.T) {
	rootfs := t.TempDir()
	registry := &RegistryDockerHub{}

	got, err := registry.joinRoot(rootfs, "/etc/passwd")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(rootfs, "etc", "passwd"), got)
}

func TestJoinRootReturnsRootForCurrentDirectory(t *testing.T) {
	rootfs := t.TempDir()
	registry := &RegistryDockerHub{}

	got, err := registry.joinRoot(rootfs, ".")

	require.NoError(t, err)
	assert.Equal(t, rootfs, got)
}

func TestJoinRootRejectsEscapingPaths(t *testing.T) {
	rootfs := t.TempDir()
	registry := &RegistryDockerHub{}

	tests := []string{
		"../escape",
		"../../escape",
		"foo/../../../escape",
	}
	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			got, err := registry.joinRoot(rootfs, tt)

			require.Error(t, err)
			assert.Empty(t, got)
			assert.True(t, strings.Contains(err.Error(), "path escapes root"))
		})
	}
}

func TestApplyOneLayerRejectsSymlinkParentEscape(t *testing.T) {
	rootfs := t.TempDir()
	outside := t.TempDir()
	layer := writeDockerHubLayer(t,
		&tar.Header{Name: "linkdir", Typeflag: tar.TypeSymlink, Linkname: outside, Mode: 0o777},
		&tar.Header{Name: "linkdir/pwned.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len("pwned\n"))},
	)

	err := (&RegistryDockerHub{}).applyOneLayer(rootfs, layer)

	require.Error(t, err)
	_, statErr := os.Stat(filepath.Join(outside, "pwned.txt"))
	assert.True(t, os.IsNotExist(statErr), "outside file should not be created")
}

func TestApplyOneLayerRejectsEscapingSymlinkTarget(t *testing.T) {
	rootfs := t.TempDir()
	layer := writeDockerHubLayer(t,
		&tar.Header{Name: "etc/link", Typeflag: tar.TypeSymlink, Linkname: "../../outside", Mode: 0o777},
	)

	err := (&RegistryDockerHub{}).applyOneLayer(rootfs, layer)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "symlink target escapes rootfs")
}

func TestApplyOneLayerAllowsInRootSymlinkParent(t *testing.T) {
	rootfs := t.TempDir()
	payload := "xxx"
	layer := writeDockerHubLayer(t,
		&tar.Header{Name: "usr", Typeflag: tar.TypeDir, Mode: 0o755},
		&tar.Header{Name: "usr/bin", Typeflag: tar.TypeDir, Mode: 0o755},
		&tar.Header{Name: "bin", Typeflag: tar.TypeSymlink, Linkname: "usr/bin", Mode: 0o777},
		&tar.Header{Name: "bin/tool", Typeflag: tar.TypeReg, Mode: 0o755, Size: int64(len(payload))},
	)

	err := (&RegistryDockerHub{}).applyOneLayer(rootfs, layer)

	require.NoError(t, err)
	got, err := os.ReadFile(filepath.Join(rootfs, "usr", "bin", "tool"))
	require.NoError(t, err)
	assert.Equal(t, payload, string(got))
}

func TestApplyOneLayerRejectsWhiteoutThroughSymlinkParent(t *testing.T) {
	rootfs := t.TempDir()
	outside := t.TempDir()
	victim := filepath.Join(outside, "victim")
	require.NoError(t, os.WriteFile(victim, []byte("safe\n"), 0o644))
	layer := writeDockerHubLayer(t,
		&tar.Header{Name: "linkdir", Typeflag: tar.TypeSymlink, Linkname: outside, Mode: 0o777},
		&tar.Header{Name: "linkdir/.wh.victim", Typeflag: tar.TypeReg, Mode: 0o644, Size: 0},
	)

	err := (&RegistryDockerHub{}).applyOneLayer(rootfs, layer)

	require.Error(t, err)
	got, readErr := os.ReadFile(victim)
	require.NoError(t, readErr)
	assert.Equal(t, "safe\n", string(got))
}

func writeDockerHubLayer(t *testing.T, headers ...*tar.Header) string {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, hdr := range headers {
		require.NoError(t, tw.WriteHeader(hdr))
		if hdr.Typeflag == tar.TypeReg || hdr.Typeflag == tar.TypeRegA {
			if hdr.Size > 0 {
				_, err := tw.Write(bytes.Repeat([]byte("x"), int(hdr.Size)))
				require.NoError(t, err)
			}
		}
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	path := filepath.Join(t.TempDir(), "layer.tar.gz")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o644))
	return path
}
