package image

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractTarToDirWithOptionsRejectsOversizedContext(t *testing.T) {
	body := buildTestTar(t, map[string]string{
		"Dripfile": strings.Repeat("a", 128),
	})

	err := ExtractTarToDirWithOptions(bytes.NewReader(body), t.TempDir(), ExtractTarOptions{
		MaxBytes:   64,
		MaxFile:    1024,
		MaxEntries: 10,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "build context too large")
}

func TestExtractTarToDirWithOptionsRejectsOversizedFile(t *testing.T) {
	body := buildTestTar(t, map[string]string{
		"Dripfile": strings.Repeat("a", 128),
	})

	err := ExtractTarToDirWithOptions(bytes.NewReader(body), t.TempDir(), ExtractTarOptions{
		MaxBytes:   int64(len(body) + 1024),
		MaxFile:    64,
		MaxEntries: 10,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "build context file too large")
}

func TestExtractTarToDirWithOptionsRejectsTooManyEntries(t *testing.T) {
	body := buildTestTar(t, map[string]string{
		"Dripfile": "FROM scratch\n",
		"a.txt":    "a",
	})

	err := ExtractTarToDirWithOptions(bytes.NewReader(body), t.TempDir(), ExtractTarOptions{
		MaxBytes:   int64(len(body) + 1024),
		MaxFile:    1024,
		MaxEntries: 1,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "too many entries")
}

func TestExtractTarToDirWithOptionsRejectsSymlinkOverwrite(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "outside-target")
	require.NoError(t, os.WriteFile(outside, []byte("safe\n"), 0o666))

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "overwrite-me",
		Typeflag: tar.TypeSymlink,
		Linkname: outside,
		Mode:     0o777,
	}))
	payload := []byte("overwritten\n")
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "overwrite-me",
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     int64(len(payload)),
	}))
	_, err := tw.Write(payload)
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	err = ExtractTarToDirWithOptions(bytes.NewReader(buf.Bytes()), t.TempDir(), ExtractTarOptions{
		MaxBytes:   int64(buf.Len() + 1024),
		MaxFile:    1024,
		MaxEntries: 10,
	})

	require.Error(t, err)
	got, readErr := os.ReadFile(outside)
	require.NoError(t, readErr)
	assert.Equal(t, "safe\n", string(got))
}

func TestExtractTarToDirWithOptionsRejectsParentDirectorySymlink(t *testing.T) {
	outside := t.TempDir()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "dir",
		Typeflag: tar.TypeSymlink,
		Linkname: outside,
		Mode:     0o777,
	}))
	payload := []byte("data\n")
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Name:     "dir/file.txt",
		Typeflag: tar.TypeReg,
		Mode:     0o644,
		Size:     int64(len(payload)),
	}))
	_, err := tw.Write(payload)
	require.NoError(t, err)
	require.NoError(t, tw.Close())

	err = ExtractTarToDirWithOptions(bytes.NewReader(buf.Bytes()), t.TempDir(), ExtractTarOptions{
		MaxBytes:   int64(buf.Len() + 1024),
		MaxFile:    1024,
		MaxEntries: 10,
	})

	require.Error(t, err)
	_, statErr := os.Stat(filepath.Join(outside, "file.txt"))
	assert.True(t, os.IsNotExist(statErr), "outside file should not be created")
}

func buildTestTar(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		b := []byte(content)
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(b)),
		}))
		_, err := tw.Write(b)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	return buf.Bytes()
}
