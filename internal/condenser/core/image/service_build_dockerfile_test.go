package image

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFromSpecSupportsAliasAndPlatform(t *testing.T) {
	got, err := parseFromSpec("--platform=linux/amd64 golang:1.22 AS builder")

	require.NoError(t, err)
	assert.Equal(t, "golang:1.22", got.image)
	assert.Equal(t, "builder", got.alias)
}

func TestParseCopySpecSupportsFromFlagAndJSONForm(t *testing.T) {
	got, err := parseCopySpec(`--from=builder --chmod=755 ["bin/app", "/usr/local/bin/app"]`)

	require.NoError(t, err)
	assert.Equal(t, "builder", got.from)
	assert.Equal(t, "755", got.chmod)
	assert.Equal(t, []string{"bin/app"}, got.sources)
	assert.Equal(t, "/usr/local/bin/app", got.dest)
}

func TestApplyCopyCopiesFromNamedBuildStage(t *testing.T) {
	stageRoot := t.TempDir()
	currentRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(stageRoot, "out"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stageRoot, "out", "app"), []byte("hello"), 0o644))

	stage := buildStage{
		name:  "builder",
		index: 0,
		state: buildState{rootfsPath: stageRoot},
	}
	state := buildState{rootfsPath: currentRoot, workdir: "/"}
	service := &ImageService{}

	err := service.applyCopy(&state, t.TempDir(), []buildStage{stage}, map[string]buildStage{"builder": stage}, "--from=builder /out/app /usr/local/bin/app")

	require.NoError(t, err)
	got, err := os.ReadFile(filepath.Join(currentRoot, "usr", "local", "bin", "app"))
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))
}

func TestApplyCopySupportsQuotedMultipleSources(t *testing.T) {
	contextDir := t.TempDir()
	currentRoot := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(contextDir, "one file.txt"), []byte("one"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(contextDir, "two.txt"), []byte("two"), 0o644))
	state := buildState{rootfsPath: currentRoot, workdir: "/app"}
	service := &ImageService{}

	err := service.applyCopy(&state, contextDir, nil, nil, `"one file.txt" two.txt ./`)

	require.NoError(t, err)
	gotOne, err := os.ReadFile(filepath.Join(currentRoot, "app", "one file.txt"))
	require.NoError(t, err)
	gotTwo, err := os.ReadFile(filepath.Join(currentRoot, "app", "two.txt"))
	require.NoError(t, err)
	assert.Equal(t, "one", string(gotOne))
	assert.Equal(t, "two", string(gotTwo))
}

func TestApplyEnvSupportsQuotedValues(t *testing.T) {
	state := newBuildState()
	service := &ImageService{}

	err := service.applyEnv(&state, `APP_NAME="hello world" LOG_LEVEL debug`)

	require.NoError(t, err)
	assert.Contains(t, state.env, "APP_NAME=hello world")
	assert.Contains(t, state.env, "LOG_LEVEL=debug")
}
