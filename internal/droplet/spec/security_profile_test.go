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

func TestResolveSecurityProfileDeploy(t *testing.T) {
	// == exercise ==
	deployProfile, err := ResolveSecurityProfile(SecurityProfileDeploy)
	defaultProfile := DefaultSecurityProfile()

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, SecurityProfileDeploy, deployProfile.Name)
	assert.NotContains(t, deployProfile.Capabilities.Base, "CAP_NET_RAW")
	assert.NotContains(t, deployProfile.Capabilities.Base, "CAP_MKNOD")
	assert.Contains(t, defaultProfile.Capabilities.Base, "CAP_NET_RAW")
	assert.Contains(t, defaultProfile.Capabilities.Base, "CAP_MKNOD")
	assert.Equal(t, defaultProfile.AppArmorProfile, deployProfile.AppArmorProfile)
	require.NotNil(t, deployProfile.Seccomp)
	assert.Equal(t, defaultProfile.Seccomp.DefaultAction, deployProfile.Seccomp.DefaultAction)
}

func TestResolveSecurityProfileRestricted(t *testing.T) {
	// == exercise ==
	restrictedProfile, err := ResolveSecurityProfile(SecurityProfileRestricted)

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, SecurityProfileRestricted, restrictedProfile.Name)
	assert.Empty(t, restrictedProfile.Capabilities.Base)
	require.NotNil(t, restrictedProfile.Seccomp)
	assert.Equal(t, "SCMP_ACT_ALLOW", restrictedProfile.Seccomp.DefaultAction)
	assert.Equal(t, "raind-default", restrictedProfile.AppArmorProfile)
}

func TestResolveSecurityProfilePrivileged(t *testing.T) {
	// == exercise ==
	privilegedProfile, err := ResolveSecurityProfile(SecurityProfilePrivileged)

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, SecurityProfilePrivileged, privilegedProfile.Name)
	assert.Contains(t, privilegedProfile.Capabilities.Base, "CAP_SYS_ADMIN")
	assert.Contains(t, privilegedProfile.Capabilities.Base, "CAP_BPF")
	assert.Nil(t, privilegedProfile.Seccomp)
	assert.Empty(t, privilegedProfile.AppArmorProfile)
}

func TestResolveSecurityProfileUnconfined(t *testing.T) {
	// == exercise ==
	unconfinedProfile, err := ResolveSecurityProfile(SecurityProfileUnconfined)
	defaultProfile := DefaultSecurityProfile()

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, SecurityProfileUnconfined, unconfinedProfile.Name)
	assert.Equal(t, defaultProfile.Capabilities.Base, unconfinedProfile.Capabilities.Base)
	assert.Nil(t, unconfinedProfile.Seccomp)
	assert.Empty(t, unconfinedProfile.AppArmorProfile)
}

func TestResolveSecurityProfileRejectsUnknownProfile(t *testing.T) {
	// == exercise ==
	_, err := ResolveSecurityProfile("unknown-profile")

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

func TestBuildSpecAcceptsResolvedSecurityOption(t *testing.T) {
	// == setup ==
	ep := uint32(1)

	// == exercise ==
	config, err := buildSpec(ConfigOptions{
		Security: SecurityOption{
			ProfileName:      "custom-resolved",
			BaseCapabilities: []string{"CAP_CHOWN"},
			Seccomp: &SeccompObject{
				DefaultAction:   "SCMP_ACT_ALLOW",
				DefaultErrnoRet: &ep,
			},
			AppArmorProfile: "raind-default",
		},
	})

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, []string{"CAP_CHOWN"}, config.Process.Capabilities.Effective)
	require.NotNil(t, config.LinuxSpec.Seccomp)
	assert.Equal(t, "SCMP_ACT_ALLOW", config.LinuxSpec.Seccomp.DefaultAction)
	assert.Equal(t, "raind-default", config.LinuxSpec.AppArmorProfile)
}

func TestBuildSpecAcceptsDeploySecurityProfile(t *testing.T) {
	// == exercise ==
	config, err := buildSpec(ConfigOptions{
		Security: SecurityOption{ProfileName: SecurityProfileDeploy},
	})

	// == assert ==
	require.NoError(t, err)
	assert.NotContains(t, config.Process.Capabilities.Effective, "CAP_NET_RAW")
	assert.NotContains(t, config.Process.Capabilities.Effective, "CAP_MKNOD")
	assert.Equal(t, DeploySecurityProfile().AppArmorProfile, config.LinuxSpec.AppArmorProfile)
	require.NotNil(t, config.LinuxSpec.Seccomp)
	assert.Equal(t, "SCMP_ACT_ALLOW", config.LinuxSpec.Seccomp.DefaultAction)
}

func TestBuildSpecAcceptsRestrictedSecurityProfile(t *testing.T) {
	// == exercise ==
	config, err := buildSpec(ConfigOptions{
		Security: SecurityOption{ProfileName: SecurityProfileRestricted},
	})

	// == assert ==
	require.NoError(t, err)
	assert.Empty(t, config.Process.Capabilities.Effective)
	assert.Equal(t, RestrictedSecurityProfile().AppArmorProfile, config.LinuxSpec.AppArmorProfile)
	require.NotNil(t, config.LinuxSpec.Seccomp)
	assert.Equal(t, "SCMP_ACT_ALLOW", config.LinuxSpec.Seccomp.DefaultAction)
}

func TestBuildSpecAcceptsPrivilegedSecurityProfile(t *testing.T) {
	// == exercise ==
	config, err := buildSpec(ConfigOptions{
		Security: SecurityOption{ProfileName: SecurityProfilePrivileged},
	})

	// == assert ==
	require.NoError(t, err)
	assert.Contains(t, config.Process.Capabilities.Effective, "CAP_SYS_ADMIN")
	assert.Contains(t, config.Process.Capabilities.Effective, "CAP_BPF")
	assert.Nil(t, config.LinuxSpec.Seccomp)
	assert.Empty(t, config.LinuxSpec.AppArmorProfile)
}

func TestBuildSpecAcceptsUnconfinedSecurityProfile(t *testing.T) {
	// == exercise ==
	config, err := buildSpec(ConfigOptions{
		Security: SecurityOption{ProfileName: SecurityProfileUnconfined},
	})

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, DefaultSecurityProfile().Capabilities.Base, config.Process.Capabilities.Effective)
	assert.Nil(t, config.LinuxSpec.Seccomp)
	assert.Empty(t, config.LinuxSpec.AppArmorProfile)
}

func TestBuildSpecRejectsUnknownSecurityProfile(t *testing.T) {
	// == exercise ==
	_, err := buildSpec(ConfigOptions{
		Security: SecurityOption{ProfileName: "unknown-profile"},
	})

	// == assert ==
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown security profile")
}
