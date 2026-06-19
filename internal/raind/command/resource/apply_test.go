package resourcecommand

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatWarning(t *testing.T) {
	assert.Equal(
		t,
		"Pod demo/web spec.unsupported: ignored",
		formatWarning("Pod", "demo", "web", "spec.unsupported", "ignored"),
	)
	assert.Equal(t, "ignored", formatWarning("", "", "", "", "ignored"))
}
