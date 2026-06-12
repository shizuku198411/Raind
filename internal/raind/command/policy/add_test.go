package policy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateType(t *testing.T) {
	for _, typ := range []string{"ew", "ns-obs", "ns-enf"} {
		got, err := validateType(typ)
		require.NoError(t, err)
		assert.Equal(t, typ, got)
	}

	_, err := validateType("invalid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "allowed type")
}
