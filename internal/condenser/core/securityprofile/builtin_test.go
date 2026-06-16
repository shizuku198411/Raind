package securityprofile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceListBuiltInProfiles(t *testing.T) {
	profiles := NewService().List()

	require.Len(t, profiles, 3)
	assert.Equal(t, ProfileDefault, profiles[0].Name)
	assert.Equal(t, ProfileDev, profiles[1].Name)
	assert.Equal(t, ProfileDeploy, profiles[2].Name)
	for _, profile := range profiles {
		assert.Equal(t, ProfileTypeBuiltIn, profile.Type)
		assert.True(t, profile.SeccompEnabled)
		assert.Equal(t, "raind-default", profile.AppArmorProfile)
	}
	assert.Equal(t, 14, profiles[0].CapabilitiesCount)
	assert.Equal(t, 14, profiles[1].CapabilitiesCount)
	assert.Equal(t, 12, profiles[2].CapabilitiesCount)
}

func TestServiceGetBuiltInProfiles(t *testing.T) {
	service := NewService()

	profile, err := service.Get(ProfileDev)
	require.NoError(t, err)
	assert.Equal(t, ProfileDev, profile.Name)
	assert.Equal(t, DefaultSecurityProfile().Capabilities.Base, profile.Capabilities.Base)
	assert.Equal(t, "SCMP_ACT_ALLOW", profile.Seccomp.DefaultAction)
}

func TestServiceGetDeployProfile(t *testing.T) {
	profile, err := NewService().Get(ProfileDeploy)
	require.NoError(t, err)

	assert.Equal(t, ProfileDeploy, profile.Name)
	assert.NotContains(t, profile.Capabilities.Base, "CAP_NET_RAW")
	assert.NotContains(t, profile.Capabilities.Base, "CAP_MKNOD")
	assert.Contains(t, profile.Capabilities.Base, "CAP_CHOWN")
	require.NotNil(t, profile.Seccomp)
	assert.Equal(t, "SCMP_ACT_ALLOW", profile.Seccomp.DefaultAction)
	assert.Equal(t, "raind-default", profile.AppArmorProfile)
}

func TestServiceRejectsUnknownProfile(t *testing.T) {
	_, err := NewService().Get("restricted")
	assert.ErrorContains(t, err, "unknown security profile")
}

func TestCloneSeccompObject(t *testing.T) {
	profile := DefaultSecurityProfile()
	cloned := CloneSeccompObject(profile.Seccomp)
	require.NotNil(t, cloned)
	require.NotSame(t, profile.Seccomp, cloned)
	require.NotNil(t, cloned.DefaultErrnoRet)
	require.NotSame(t, profile.Seccomp.DefaultErrnoRet, cloned.DefaultErrnoRet)

	cloned.Syscalls[0].Names[0] = "changed"
	assert.Equal(t, "bpf", profile.Seccomp.Syscalls[0].Names[0])
}
