package pathenv

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOrDefaultReturnsPATHValue(t *testing.T) {
	assert.Equal(t, "/custom/bin:/bin", OrDefault([]string{"HOME=/root", "PATH=/custom/bin:/bin"}))
}

func TestOrDefaultReturnsDefaultWhenPATHMissingOrEmpty(t *testing.T) {
	assert.Equal(t, Default, OrDefault(nil))
	assert.Equal(t, Default, OrDefault([]string{"HOME=/root"}))
	assert.Equal(t, Default, OrDefault([]string{"PATH="}))
}
