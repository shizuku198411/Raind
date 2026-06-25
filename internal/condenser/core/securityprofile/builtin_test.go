package securityprofile

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceListBuiltInProfiles(t *testing.T) {
	profiles := NewServiceWithStoreDir(t.TempDir()).List()

	require.Len(t, profiles, 6)
	assert.Equal(t, ProfileDefault, profiles[0].Name)
	assert.Equal(t, ProfileDev, profiles[1].Name)
	assert.Equal(t, ProfileDeploy, profiles[2].Name)
	assert.Equal(t, ProfileRestricted, profiles[3].Name)
	assert.Equal(t, ProfilePrivileged, profiles[4].Name)
	assert.Equal(t, ProfileUnconfined, profiles[5].Name)
	for _, profile := range profiles {
		assert.Equal(t, ProfileTypeBuiltIn, profile.Type)
	}
	assert.True(t, profiles[0].SeccompEnabled)
	assert.True(t, profiles[1].SeccompEnabled)
	assert.True(t, profiles[2].SeccompEnabled)
	assert.True(t, profiles[3].SeccompEnabled)
	assert.False(t, profiles[4].SeccompEnabled)
	assert.False(t, profiles[5].SeccompEnabled)
	assert.Equal(t, "raind-default", profiles[0].AppArmorProfile)
	assert.Equal(t, "raind-default", profiles[1].AppArmorProfile)
	assert.Equal(t, "raind-default", profiles[2].AppArmorProfile)
	assert.Equal(t, "raind-default", profiles[3].AppArmorProfile)
	assert.Empty(t, profiles[4].AppArmorProfile)
	assert.Empty(t, profiles[5].AppArmorProfile)
	assert.False(t, profiles[0].NoNewPrivileges)
	assert.True(t, profiles[1].NoNewPrivileges)
	assert.True(t, profiles[2].NoNewPrivileges)
	assert.True(t, profiles[3].NoNewPrivileges)
	assert.False(t, profiles[4].NoNewPrivileges)
	assert.False(t, profiles[5].NoNewPrivileges)
	assert.Equal(t, 14, profiles[0].CapabilitiesCount)
	assert.Equal(t, 14, profiles[1].CapabilitiesCount)
	assert.Equal(t, 12, profiles[2].CapabilitiesCount)
	assert.Equal(t, 0, profiles[3].CapabilitiesCount)
	assert.Equal(t, len(allCapabilities()), profiles[4].CapabilitiesCount)
	assert.Equal(t, 14, profiles[5].CapabilitiesCount)
}

func TestServiceGetBuiltInProfiles(t *testing.T) {
	service := NewServiceWithStoreDir(t.TempDir())

	profile, err := service.Get(ProfileDev)
	require.NoError(t, err)
	assert.Equal(t, ProfileDev, profile.Name)
	assert.Equal(t, DefaultSecurityProfile().Capabilities.Base, profile.Capabilities.Base)
	assert.Equal(t, "SCMP_ACT_ALLOW", profile.Seccomp.DefaultAction)
	assert.True(t, profile.NoNewPrivileges)
}

func TestServiceGetDeployProfile(t *testing.T) {
	profile, err := NewServiceWithStoreDir(t.TempDir()).Get(ProfileDeploy)
	require.NoError(t, err)

	assert.Equal(t, ProfileDeploy, profile.Name)
	assert.NotContains(t, profile.Capabilities.Base, "CAP_NET_RAW")
	assert.NotContains(t, profile.Capabilities.Base, "CAP_MKNOD")
	assert.Contains(t, profile.Capabilities.Base, "CAP_CHOWN")
	require.NotNil(t, profile.Seccomp)
	assert.Equal(t, "SCMP_ACT_ALLOW", profile.Seccomp.DefaultAction)
	assert.Equal(t, "raind-default", profile.AppArmorProfile)
	assert.True(t, profile.NoNewPrivileges)
}

func TestServiceGetRestrictedProfile(t *testing.T) {
	profile, err := NewServiceWithStoreDir(t.TempDir()).Get(ProfileRestricted)
	require.NoError(t, err)

	assert.Equal(t, ProfileRestricted, profile.Name)
	assert.Empty(t, profile.Capabilities.Base)
	require.NotNil(t, profile.Seccomp)
	assert.Equal(t, "SCMP_ACT_ALLOW", profile.Seccomp.DefaultAction)
	assert.Equal(t, "raind-default", profile.AppArmorProfile)
	assert.True(t, profile.NoNewPrivileges)
}

func TestServiceGetPrivilegedProfile(t *testing.T) {
	profile, err := NewServiceWithStoreDir(t.TempDir()).Get(ProfilePrivileged)
	require.NoError(t, err)

	assert.Equal(t, ProfilePrivileged, profile.Name)
	assert.Contains(t, profile.Capabilities.Base, "CAP_SYS_ADMIN")
	assert.Contains(t, profile.Capabilities.Base, "CAP_BPF")
	assert.Nil(t, profile.Seccomp)
	assert.Empty(t, profile.AppArmorProfile)
	assert.False(t, profile.NoNewPrivileges)
}

func TestServiceGetUnconfinedProfile(t *testing.T) {
	profile, err := NewServiceWithStoreDir(t.TempDir()).Get(ProfileUnconfined)
	require.NoError(t, err)

	assert.Equal(t, ProfileUnconfined, profile.Name)
	assert.Equal(t, DefaultSecurityProfile().Capabilities.Base, profile.Capabilities.Base)
	assert.Nil(t, profile.Seccomp)
	assert.Empty(t, profile.AppArmorProfile)
	assert.False(t, profile.NoNewPrivileges)
}

func TestServiceRejectsUnknownProfile(t *testing.T) {
	_, err := NewServiceWithStoreDir(t.TempDir()).Get("unknown-profile")
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

func TestServiceRegisterCustomProfile(t *testing.T) {
	service := NewServiceWithStoreDir(t.TempDir())

	profile, err := service.Register(CustomProfileManifest{
		APIVersion: "raind.io/v1",
		Kind:       "SecurityProfile",
		Metadata: CustomProfileMetadata{
			Name: "custom-dev",
		},
		Spec: CustomProfileSpec{
			Extends: "dev",
			AddCap:  []string{"CAP_SYS_PTRACE"},
			DropCap: []string{"CAP_NET_RAW"},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "custom-dev", profile.Name)
	assert.Equal(t, ProfileTypeCustom, profile.Type)
	assert.Equal(t, ProfileDev, profile.Extends)
	assert.Contains(t, profile.Capabilities.Base, "CAP_SYS_PTRACE")
	assert.NotContains(t, profile.Capabilities.Base, "CAP_NET_RAW")
	assert.Contains(t, profile.Capabilities.Base, "CAP_CHOWN")
	assert.True(t, profile.NoNewPrivileges)

	resolved, err := service.Resolve("custom-dev")
	require.NoError(t, err)
	assert.Equal(t, profile.Capabilities.Base, resolved.Capabilities.Base)

	profiles := service.List()
	require.Len(t, profiles, 7)
	assert.Equal(t, "custom-dev", profiles[6].Name)
	assert.Equal(t, ProfileTypeCustom, profiles[6].Type)
}

func TestServiceRegisterRequiresExtends(t *testing.T) {
	_, err := NewServiceWithStoreDir(t.TempDir()).Register(CustomProfileManifest{Name: "custom-dev"})
	assert.ErrorContains(t, err, "extends is required")
}

func TestServiceRegisterRejectsBuiltInName(t *testing.T) {
	_, err := NewServiceWithStoreDir(t.TempDir()).Register(CustomProfileManifest{Name: ProfileDev, Extends: ProfileDefault})
	assert.ErrorContains(t, err, "built-in")
}

func TestServiceDeleteCustomProfile(t *testing.T) {
	service := NewServiceWithStoreDir(t.TempDir())
	_, err := service.Register(CustomProfileManifest{Name: "custom-dev", Extends: ProfileDev, AddCap: []string{"CAP_SYS_PTRACE"}})
	require.NoError(t, err)

	require.NoError(t, service.Delete("custom-dev"))
	_, err = service.Get("custom-dev")
	assert.ErrorContains(t, err, "unknown security profile")
}

func TestServiceDeleteRejectsBuiltInProfile(t *testing.T) {
	err := NewServiceWithStoreDir(t.TempDir()).Delete(ProfileDefault)
	assert.ErrorContains(t, err, "cannot delete built-in")
}
