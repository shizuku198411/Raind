package podcommand

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseKeyValueSlice(t *testing.T) {
	got, err := parseKeyValueSlice([]string{"app=web", "tier=frontend"})

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"app": "web", "tier": "frontend"}, got)

	_, err = parseKeyValueSlice([]string{"missing-equals"})
	require.Error(t, err)
}
