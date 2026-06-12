package containercommand

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateVolumeFlag(t *testing.T) {
	got, err := validateVolumeFlag([]string{"/host:/container"})

	require.NoError(t, err)
	assert.Equal(t, []string{"/host:/container"}, got)

	_, err = validateVolumeFlag([]string{"/host-only"})
	require.Error(t, err)
}

func TestValidatePublishFlag(t *testing.T) {
	got, err := validatePublishFlag([]string{"8080:80", "8443:443:tcp"})

	require.NoError(t, err)
	assert.Equal(t, []string{"8080:80", "8443:443:tcp"}, got)

	_, err = validatePublishFlag([]string{"8080"})
	require.Error(t, err)
}

func TestValidateDeviceFlag(t *testing.T) {
	got, err := validateDeviceFlag([]string{"/dev/null", "/dev/fuse:/dev/fuse:rwm"})

	require.NoError(t, err)
	assert.Equal(t, []string{"/dev/null", "/dev/fuse:/dev/fuse:rwm"}, got)

	_, err = validateDeviceFlag([]string{":/dev/null"})
	require.Error(t, err)
}
