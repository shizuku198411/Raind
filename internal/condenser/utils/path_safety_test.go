package utils

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSafeJoinAllowsChildPath(t *testing.T) {
	got, err := SafeJoin("/runtime/root", "namespace", "id")

	require.NoError(t, err)
	assert.Equal(t, filepath.Join("/runtime/root", "namespace", "id"), got)
}

func TestSafeJoinRejectsEscapingPath(t *testing.T) {
	_, err := SafeJoin("/runtime/root", "..", "other")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "path escapes root")
}

func TestSafeJoinRejectsRootItself(t *testing.T) {
	_, err := SafeJoin("/runtime/root", "")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "path escapes root")
}

func TestEnsurePathUnderRootRejectsSiblingPrefix(t *testing.T) {
	err := EnsurePathUnderRoot("/runtime/root", "/runtime/root-other/file")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "path escapes root")
}
