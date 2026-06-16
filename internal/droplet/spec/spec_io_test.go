package spec

import (
	"path/filepath"
	"raind/internal/droplet/oci"
	"raind/internal/droplet/utils"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfigOptions() ConfigOptions {
	timeout := 3
	return ConfigOptions{
		Rootfs: "/rootfs",
		Mounts: []MountOption{
			{Source: "/host/data", Destination: "/data", Type: "bind", Options: []string{"rbind", "rprivate"}},
		},
		Process: ProcessOption{
			Cwd:     "/work",
			Env:     []string{"PATH=/custom/bin", "APP_ENV=test"},
			Args:    []string{"/bin/sh", "-c", "echo hi"},
			CapAdd:  []string{"net_admin", "CAP_SYS_TIME", "net_admin"},
			CapDrop: []string{"CAP_NET_RAW"},
		},
		Namespace: []NamespaceOption{
			{Type: "mount"},
			{Type: "network", Path: "/proc/1/ns/net"},
		},
		Hostname: "container-host",
		Net: NetOption{
			HostInterface:       "veth-host",
			BridgeInterfaceName: "raind0",
			InterfaceName:       "eth0",
			Address:             "10.166.0.2/24",
			Gateway:             "10.166.0.1",
			Dns:                 []string{"1.1.1.1"},
		},
		Image: ImageOption{
			ImageLayer: []string{"/layers/a", "/layers/b"},
			UpperDir:   "/upper",
			WorkDir:    "/work",
		},
		Hooks: HookLifecycleOption{
			CreateRuntime: []HookOption{{Path: "/bin/create-runtime", Args: []string{"a"}, Env: []string{"A=1"}, Timeout: &timeout}},
			Poststop:      []HookOption{{Path: "/bin/poststop"}},
		},
	}
}

func TestCreateConfigFileAndLoadConfigFileRoundTrip(t *testing.T) {
	// == setup ==
	path := filepath.Join(t.TempDir(), "config.json")
	opts := testConfigOptions()

	// == exercise ==
	require.NoError(t, CreateConfigFile(path, opts))
	got, err := LoadConfigFile(path)

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, oci.OCIVersion, got.OciVersion)
	assert.Equal(t, "/rootfs", got.Root.Path)
	assert.Equal(t, []string{"/bin/sh", "-c", "echo hi"}, got.Process.Args)
	assert.Equal(t, "container-host", got.Hostname)
	assert.Len(t, got.Mounts, 1)
	assert.Len(t, got.Hooks.CreateRuntime, 1)
	assert.Len(t, got.Hooks.Poststop, 1)
	assert.NotEmpty(t, got.Annotations.Net)
	assert.NotEmpty(t, got.Annotations.Image)
}

func TestBuildProcessEnvSpecAddsDefaultsAndPreservesOverrides(t *testing.T) {
	// == exercise ==
	env := buildProcessEnvSpec([]string{"PATH=/custom/bin", "APP_ENV=test", "HOME="})

	// == assert ==
	assert.Contains(t, env, "PATH=/custom/bin")
	assert.Contains(t, env, "APP_ENV=test")
	assert.Contains(t, env, "HOME=/root")
	assert.Contains(t, env, "TERM=xterm-256color")
	assert.Contains(t, env, "LANG=C.UTF-8")
}

func TestNormalizeAndMergeCapabilities(t *testing.T) {
	// == exercise ==
	normalized := normalizeCapabilityNames([]string{" net_admin ", "CAP_NET_ADMIN", "sys_time", ""})
	merged := mergeCapabilities(
		[]string{"CAP_NET_RAW", "CAP_CHOWN"},
		[]string{"CAP_SYS_ADMIN", "CAP_NET_RAW"},
		[]string{"CAP_NET_RAW"},
	)

	// == assert ==
	assert.Equal(t, []string{"CAP_NET_ADMIN", "CAP_SYS_TIME"}, normalized)
	assert.Equal(t, []string{"CAP_CHOWN", "CAP_SYS_ADMIN"}, merged)
}

func TestBuildLinuxSpecDefaultsAndNamespaces(t *testing.T) {
	// == exercise ==
	profile := DefaultSecurityProfile()
	linuxSpec := buildLinuxSpec(ConfigOptions{
		Namespace: []NamespaceOption{
			{Type: "mount"},
			{Type: "network", Path: "/proc/1/ns/net"},
		},
	}, profile)

	// == assert ==
	assert.Equal(t, 1073741824, linuxSpec.Resources.Memory.Limit)
	assert.Equal(t, 100000, linuxSpec.Resources.Cpu.Period)
	assert.Equal(t, 80000, linuxSpec.Resources.Cpu.Quota)
	require.NotNil(t, linuxSpec.Seccomp)
	assert.Equal(t, "SCMP_ACT_ALLOW", linuxSpec.Seccomp.DefaultAction)
	assert.Equal(t, profile.AppArmorProfile, linuxSpec.AppArmorProfile)
	assert.Equal(t, []NamespaceObject{
		{Type: "mount"},
		{Type: "network", Path: "/proc/1/ns/net"},
	}, linuxSpec.Namespaces)
}

func TestBuildAnnotationSpecContainsNetAndImageJSON(t *testing.T) {
	// == exercise ==
	annotation := buildAnnotationSpec(testConfigOptions())

	// == assert ==
	assert.Equal(t, oci.AnnotationVersion, annotation.Version)
	var netConfig NetConfigObject
	require.NoError(t, utils.StringToJson(annotation.Net, &netConfig))
	assert.Equal(t, "veth-host", netConfig.HostInterface)
	assert.Equal(t, "10.166.0.2/24", netConfig.Interface.IPv4.Address)
	var imageConfig ImageConfigObject
	require.NoError(t, utils.StringToJson(annotation.Image, &imageConfig))
	assert.Equal(t, []string{"/layers/a", "/layers/b"}, imageConfig.ImageLayer)
}
