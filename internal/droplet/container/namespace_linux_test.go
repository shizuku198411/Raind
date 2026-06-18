package container

import (
	"os"

	"raind/internal/droplet/spec"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildNamespaceConfigCreatesNamespacesWithoutPath(t *testing.T) {
	// == setup ==
	containerSpec := spec.Spec{
		LinuxSpec: spec.LinuxSpecObject{
			Namespaces: []spec.NamespaceObject{
				{Type: "mount"},
				{Type: "network"},
				{Type: "uts"},
				{Type: "pid"},
				{Type: "ipc"},
				{Type: "user"},
				{Type: "cgroup"},
			},
		},
	}

	// == exercise ==
	nsConfig := buildNamespaceConfig(containerSpec)

	// == assert ==
	assert.True(t, nsConfig.mount)
	assert.True(t, nsConfig.network)
	assert.True(t, nsConfig.uts)
	assert.True(t, nsConfig.pid)
	assert.True(t, nsConfig.ipc)
	assert.True(t, nsConfig.user)
	assert.True(t, nsConfig.cgroup)
}

func TestBuildNamespaceConfigUsesPathAsJoinTarget(t *testing.T) {
	// == setup ==
	containerSpec := spec.Spec{
		LinuxSpec: spec.LinuxSpecObject{
			Namespaces: []spec.NamespaceObject{
				{Type: "network", Path: "/proc/1/ns/net"},
				{Type: "ipc", Path: "/proc/1/ns/ipc"},
				{Type: "uts", Path: "/proc/1/ns/uts"},
			},
		},
	}

	// == exercise ==
	nsConfig := buildNamespaceConfig(containerSpec)

	// == assert ==
	assert.False(t, nsConfig.network)
	assert.False(t, nsConfig.ipc)
	assert.False(t, nsConfig.uts)
	assert.Equal(t, "/proc/1/ns/net", nsConfig.networkPath)
	assert.Equal(t, "/proc/1/ns/ipc", nsConfig.ipcPath)
	assert.Equal(t, "/proc/1/ns/uts", nsConfig.utsPath)
}

func TestBuildCloneFlagsMapsEnabledNamespaces(t *testing.T) {
	// == setup ==
	nsConfig := namespaceConfig{
		mount:   true,
		network: true,
		uts:     true,
		pid:     true,
		ipc:     true,
		user:    true,
		cgroup:  true,
	}

	// == exercise ==
	flags := buildCloneFlags(nsConfig)

	// == assert ==
	assert.Equal(t, uintptr(syscall.CLONE_NEWNS|syscall.CLONE_NEWNET|syscall.CLONE_NEWUTS|syscall.CLONE_NEWPID|syscall.CLONE_NEWIPC|syscall.CLONE_NEWUSER|syscall.CLONE_NEWCGROUP), flags)
}

func TestBuildNamespaceJoinTargetsIncludesPathNamespacesInStableOrder(t *testing.T) {
	// == setup ==
	containerSpec := spec.Spec{
		LinuxSpec: spec.LinuxSpecObject{
			Namespaces: []spec.NamespaceObject{
				{Type: "uts", Path: "/proc/1/ns/uts"},
				{Type: "network", Path: "/proc/1/ns/net"},
				{Type: "ipc", Path: "/proc/1/ns/ipc"},
				{Type: "mount", Path: "/proc/1/ns/mnt"},
			},
		},
	}

	// == exercise ==
	targets := buildNamespaceJoinTargets(containerSpec)

	// == assert ==
	assert.Equal(t, []namespaceJoinTarget{
		{name: "network", path: "/proc/1/ns/net"},
		{name: "ipc", path: "/proc/1/ns/ipc"},
		{name: "uts", path: "/proc/1/ns/uts"},
	}, targets)
}

func TestUserNamespacePathReturnsPathNamespace(t *testing.T) {
	containerSpec := spec.Spec{
		LinuxSpec: spec.LinuxSpecObject{
			Namespaces: []spec.NamespaceObject{
				{Type: "user", Path: "/proc/1/ns/user"},
			},
		},
	}

	assert.Equal(t, "/proc/1/ns/user", userNamespacePath(containerSpec))
}

func TestBuildRootUserNamespaceIDMapReturnsIdentityMapOnlyWhenUserNamespaceEnabled(t *testing.T) {
	// == exercise ==
	uidMap, gidMap := buildRootUserNamespaceIDMap(namespaceConfig{user: true})
	noUIDMap, noGIDMap := buildRootUserNamespaceIDMap(namespaceConfig{})

	// == assert ==
	assert.Equal(t, []syscall.SysProcIDMap{{ContainerID: 0, HostID: 0, Size: 65535}}, uidMap)
	assert.Equal(t, []syscall.SysProcIDMap{{ContainerID: 0, HostID: 0, Size: 65535}}, gidMap)
	assert.Nil(t, noUIDMap)
	assert.Nil(t, noGIDMap)
}

func TestBuildSysProcAttrCopiesProcAttrFields(t *testing.T) {
	// == setup ==
	credential := &syscall.Credential{Uid: 0, Gid: 0, NoSetGroups: true}
	procAttr := procAttr{
		cloneFlags:    uintptr(syscall.CLONE_NEWNET),
		uidMap:        []syscall.SysProcIDMap{{ContainerID: 0, HostID: 1000, Size: 1}},
		gidMap:        []syscall.SysProcIDMap{{ContainerID: 0, HostID: 1000, Size: 1}},
		setGroupsFlag: true,
		credential:    credential,
	}

	// == exercise ==
	sysProcAttr := buildSysProcAttr(procAttr)

	// == assert ==
	assert.Equal(t, uintptr(syscall.CLONE_NEWNET), sysProcAttr.Cloneflags)
	assert.Equal(t, procAttr.uidMap, sysProcAttr.UidMappings)
	assert.Equal(t, procAttr.gidMap, sysProcAttr.GidMappings)
	assert.True(t, sysProcAttr.GidMappingsEnableSetgroups)
	assert.Same(t, credential, sysProcAttr.Credential)
}

func TestBuildProcAttrForRootlessContainerStartsAsUserNamespaceRoot(t *testing.T) {
	// == setup ==
	containerSpec := spec.Spec{
		LinuxSpec: spec.LinuxSpecObject{
			Namespaces: []spec.NamespaceObject{
				{Type: "user"},
			},
		},
		Annotations: spec.AnnotationObject{
			Rootless: `{"enabled":true}`,
		},
	}

	// == exercise ==
	procAttr := buildProcAttrForContainer(containerSpec)

	// == assert ==
	assert.Equal(t, []syscall.SysProcIDMap{{ContainerID: 0, HostID: 100000, Size: 65536}}, procAttr.uidMap)
	assert.Equal(t, []syscall.SysProcIDMap{{ContainerID: 0, HostID: 100000, Size: 65536}}, procAttr.gidMap)
	assert.False(t, procAttr.setGroupsFlag)
	if assert.NotNil(t, procAttr.credential) {
		assert.Equal(t, uint32(0), procAttr.credential.Uid)
		assert.Equal(t, uint32(0), procAttr.credential.Gid)
		assert.True(t, procAttr.credential.NoSetGroups)
	}
}

func TestBuildRootlessUserNamespaceIDMapMapsContainerRootToLoginUser(t *testing.T) {
	// == setup ==
	t.Setenv("RAIND_ROOTLESS_UID_BASE", "200000")
	t.Setenv("RAIND_ROOTLESS_GID_BASE", "300000")
	t.Setenv("RAIND_ROOTLESS_ID_MAP_SIZE", "65536")

	// == exercise ==
	uidMap, gidMap := buildRootlessUserNamespaceIDMap(namespaceConfig{user: true}, spec.RootlessConfigObject{
		Enabled: true,
		Mode:    spec.RootlessModeLoginRoot,
	})

	// == assert ==
	assert.Equal(t, []syscall.SysProcIDMap{
		{ContainerID: 0, HostID: os.Getuid(), Size: 1},
		{ContainerID: 1, HostID: 200000, Size: 65535},
	}, uidMap)
	assert.Equal(t, []syscall.SysProcIDMap{
		{ContainerID: 0, HostID: os.Getgid(), Size: 1},
		{ContainerID: 1, HostID: 300000, Size: 65535},
	}, gidMap)
}
