package namespace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateNamespaceName(t *testing.T) {
	require.NoError(t, validateNamespaceName("dev"))
	require.NoError(t, validateNamespaceName("team-a"))

	require.Error(t, validateNamespaceName(""))
	require.Error(t, validateNamespaceName("TeamA"))
	require.Error(t, validateNamespaceName("-dev"))
	require.Error(t, validateNamespaceName("dev-"))
}

func TestBridgeNameForNamespaceIsStableAndFitsLinuxInterfaceLimit(t *testing.T) {
	first := bridgeNameForNamespace("dev")
	second := bridgeNameForNamespace("dev")

	assert.Equal(t, first, second)
	assert.LessOrEqual(t, len(first), 15)
	assert.Contains(t, first, "rns")
}
