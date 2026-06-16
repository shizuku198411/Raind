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

func TestBuildSpecRejectsUnknownSecurityProfile(t *testing.T) {
	// == exercise ==
	_, err := buildSpec(ConfigOptions{
		Security: SecurityOption{ProfileName: "deploy"},
	})

	// == assert ==
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown security profile")
}
