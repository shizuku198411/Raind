package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveSecurityProfileDefaults(t *testing.T) {
	// == exercise ==
	emptyProfile, err := ResolveSecurityProfile("")
	require.NoError(t, err)
	explicitProfile, err := ResolveSecurityProfile(SecurityProfileDefault)
	require.NoError(t, err)

	// == assert ==
	assert.Equal(t, SecurityProfileDefault, emptyProfile.Name)
	assert.Equal(t, SecurityProfileDefault, explicitProfile.Name)
	assert.Equal(t, emptyProfile.Capabilities.Base, explicitProfile.Capabilities.Base)
	assert.Equal(t, emptyProfile.AppArmorProfile, explicitProfile.AppArmorProfile)
	require.NotNil(t, emptyProfile.Seccomp)
	assert.Equal(t, "SCMP_ACT_ALLOW", emptyProfile.Seccomp.DefaultAction)
}

func TestResolveSecurityProfileDev(t *testing.T) {
	// == exercise ==
	devProfile, err := ResolveSecurityProfile(SecurityProfileDev)
	defaultProfile := DefaultSecurityProfile()

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, SecurityProfileDev, devProfile.Name)
	assert.Equal(t, defaultProfile.Capabilities.Base, devProfile.Capabilities.Base)
	assert.Equal(t, defaultProfile.AppArmorProfile, devProfile.AppArmorProfile)
	require.NotNil(t, devProfile.Seccomp)
	assert.Equal(t, defaultProfile.Seccomp.DefaultAction, devProfile.Seccomp.DefaultAction)
	assert.Equal(t, defaultProfile.Seccomp.Syscalls, devProfile.Seccomp.Syscalls)
}

func TestResolveSecurityProfileRejectsUnknownProfile(t *testing.T) {
	// == exercise ==
	_, err := ResolveSecurityProfile("deploy")

	// == assert ==
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown security profile")
}

func TestCloneSeccompObjectDeepCopiesProfileSeccomp(t *testing.T) {
	// == setup ==
	profile := DefaultSecurityProfile()

	// == exercise ==
	cloned := cloneSeccompObject(profile.Seccomp)
	cloned.DefaultAction = "SCMP_ACT_ERRNO"
	cloned.Architectures[0] = "changed"
	cloned.Syscalls[0].Names[0] = "changed"
	*cloned.Syscalls[0].ErrnoRet = 99

	// == assert ==
	assert.Equal(t, "SCMP_ACT_ALLOW", profile.Seccomp.DefaultAction)
	assert.NotEqual(t, "changed", profile.Seccomp.Architectures[0])
	assert.Equal(t, "bpf", profile.Seccomp.Syscalls[0].Names[0])
	assert.Equal(t, uint32(1), *profile.Seccomp.Syscalls[0].ErrnoRet)
}

func TestBuildSpecAcceptsDevSecurityProfile(t *testing.T) {
	// == exercise ==
	config, err := buildSpec(ConfigOptions{
		Security: SecurityOption{ProfileName: SecurityProfileDev},
	})

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, DevSecurityProfile().AppArmorProfile, config.LinuxSpec.AppArmorProfile)
	require.NotNil(t, config.LinuxSpec.Seccomp)
	assert.Equal(t, "SCMP_ACT_ALLOW", config.LinuxSpec.Seccomp.DefaultAction)
}

func TestBuildSpecRejectsUnknownSecurityProfile(t *testing.T) {
	// == exercise ==
	_, err := buildSpec(ConfigOptions{
		Security: SecurityOption{ProfileName: "deploy"},
	})

	// == assert ==
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown security profile")
}
