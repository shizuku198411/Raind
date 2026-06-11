package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type jsonRoundTripObject struct {
	Name string `json:"name"`
	Num  int    `json:"num"`
}

func TestSha256BytesAndFile(t *testing.T) {
	// == setup ==
	path := filepath.Join(t.TempDir(), "data.txt")
	require.NoError(t, os.WriteFile(path, []byte("hello"), 0644))

	// == exercise ==
	bytesHash := Sha256Bytes([]byte("hello"))
	fileHash, err := Sha256File(path)

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", bytesHash)
	assert.Equal(t, bytesHash, fileHash)
}

func TestJsonHelpersRoundTrip(t *testing.T) {
	// == setup ==
	obj := jsonRoundTripObject{Name: "raind", Num: 7}
	path := filepath.Join(t.TempDir(), "data.json")

	// == exercise ==
	jsonText, err := JsonToString(obj)
	require.NoError(t, err)
	var fromString jsonRoundTripObject
	require.NoError(t, StringToJson(jsonText, &fromString))
	require.NoError(t, WriteJsonToFile(path, obj))
	var fromFile jsonRoundTripObject
	require.NoError(t, ReadJsonFile(path, &fromFile))

	// == assert ==
	assert.JSONEq(t, `{"name":"raind","num":7}`, jsonText)
	assert.Equal(t, obj, fromString)
	assert.Equal(t, obj, fromFile)
}

func TestPathHelpersUseRootDirOverride(t *testing.T) {
	// == setup ==
	root := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", root)

	// == assert ==
	assert.Equal(t, filepath.Join(root, "container-1"), ContainerDir("container-1"))
	assert.Equal(t, filepath.Join(root, "container-1", "config.json"), ConfigFilePath("container-1"))
	assert.Equal(t, filepath.Join(root, "container-1", "config_hash.json"), ConfigFileHashPath("container-1"))
	assert.Equal(t, filepath.Join(root, "container-1", "state.json"), ContainerStatePath("container-1"))
	assert.Equal(t, filepath.Join(root, "container-1", "exec.fifo"), FifoPath("container-1"))
	assert.Equal(t, filepath.Join(root, "container-1", "tty.sock"), SockPath("container-1"))
	assert.Equal(t, filepath.Join(root, "container-1", "exec_tty.sock"), ExecSockPath("container-1"))
	assert.Equal(t, filepath.Join(root, "container-1", "logs", "shim.log"), ShimLogPath("container-1"))
	assert.Equal(t, filepath.Join("/sys/fs/cgroup/raind", "container-1"), CgroupPath("container-1"))
}
