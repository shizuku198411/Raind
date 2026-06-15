package container

import (
	"os"
	"path/filepath"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeInitSyscallHandler struct {
	utils.KernelSyscallHandler

	setresgidCalls [][3]int
	setresuidCalls [][3]int
	hostnames      []string
	envs           []string
	mounts         []initMountCall
	mkdirs         []string
	mkdirAlls      []string
	openFiles      []string
	pivotRoots     [][2]string
	chdirs         []string
	unmounts       []string
	rmdirs         []string
	creates        []string
	symlinks       [][2]string
	existing       map[string]bool
}

type initMountCall struct {
	source string
	target string
	fstype string
	flags  uintptr
	data   string
}

func (f *fakeInitSyscallHandler) Setresgid(rgid int, egid int, sgid int) error {
	f.setresgidCalls = append(f.setresgidCalls, [3]int{rgid, egid, sgid})
	return nil
}

func (f *fakeInitSyscallHandler) Setresuid(ruid int, euid int, suid int) error {
	f.setresuidCalls = append(f.setresuidCalls, [3]int{ruid, euid, suid})
	return nil
}

func (f *fakeInitSyscallHandler) Sethostname(p []byte) error {
	f.hostnames = append(f.hostnames, string(p))
	return nil
}

func (f *fakeInitSyscallHandler) Setenv(key string, value string) error {
	f.envs = append(f.envs, key+"="+value)
	return nil
}

func (f *fakeInitSyscallHandler) Mount(source string, target string, fstype string, flags uintptr, data string) error {
	f.mounts = append(f.mounts, initMountCall{source: source, target: target, fstype: fstype, flags: flags, data: data})
	return nil
}

func (f *fakeInitSyscallHandler) Mkdir(path string, mode uint32) error {
	f.mkdirs = append(f.mkdirs, path)
	return nil
}

func (f *fakeInitSyscallHandler) MkdirAll(path string, perm os.FileMode) error {
	f.mkdirAlls = append(f.mkdirAlls, path)
	return nil
}

func (f *fakeInitSyscallHandler) PivotRoot(newroot string, putold string) error {
	f.pivotRoots = append(f.pivotRoots, [2]string{newroot, putold})
	return nil
}

func (f *fakeInitSyscallHandler) Chdir(path string) error {
	f.chdirs = append(f.chdirs, path)
	return nil
}

func (f *fakeInitSyscallHandler) Unmount(target string, flags int) error {
	f.unmounts = append(f.unmounts, target)
	return nil
}

func (f *fakeInitSyscallHandler) Rmdir(path string) error {
	f.rmdirs = append(f.rmdirs, path)
	return nil
}

func (f *fakeInitSyscallHandler) Stat(name string) (os.FileInfo, error) {
	if f.existing != nil && f.existing[name] {
		tmp, err := os.CreateTemp("", "raind-init-stat-*")
		if err != nil {
			return nil, err
		}
		defer tmp.Close()
		return tmp.Stat()
	}
	return nil, os.ErrNotExist
}

func (f *fakeInitSyscallHandler) IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

func (f *fakeInitSyscallHandler) Create(name string) (*os.File, error) {
	f.creates = append(f.creates, name)
	return os.CreateTemp("", "raind-init-create-*")
}

func (f *fakeInitSyscallHandler) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	f.openFiles = append(f.openFiles, name)
	return os.CreateTemp("", "raind-init-openfile-*")
}

func (f *fakeInitSyscallHandler) Lstat(name string) (os.FileInfo, error) {
	if f.existing != nil && f.existing[name] {
		tmp, err := os.CreateTemp("", "raind-init-lstat-*")
		if err != nil {
			return nil, err
		}
		defer tmp.Close()
		return tmp.Stat()
	}
	return nil, os.ErrNotExist
}

func (f *fakeInitSyscallHandler) Symlink(oldname string, newname string) error {
	f.symlinks = append(f.symlinks, [2]string{oldname, newname})
	return nil
}

func TestContainerInitSpecSecureLoadValidatesHashLoadsSpecAndRemovesHash(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	containerSpec := minimalCreateSpec()
	setupCreateSpecFile(t, containerId, containerSpec)
	hash, err := utils.Sha256File(utils.ConfigFilePath(containerId))
	require.NoError(t, err)
	require.NoError(t, utils.WriteJsonToFile(utils.ConfigFileHashPath(containerId), spec.SpecHash{Sha256: hash}))
	init := &ContainerInit{specLoader: &fakeDeleteSpecLoader{spec: containerSpec}}

	// == exercise ==
	got, err := init.specSecureLoad(containerId)

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, containerSpec, got)
	assert.NoFileExists(t, utils.ConfigFileHashPath(containerId))
}

func TestShouldIgnoreSpecHashRemoveErrorReturnsTrueForRootlessPermissionError(t *testing.T) {
	// == setup ==
	containerSpec := minimalCreateSpec()
	containerSpec.Annotations.Rootless = `{"enabled":true}`
	err := &os.PathError{Op: "remove", Path: "/etc/raind/container/container-1/config_hash.json", Err: syscall.EACCES}

	// == exercise ==
	got := shouldIgnoreSpecHashRemoveError(containerSpec, err)

	// == assert ==
	assert.True(t, got)
}

func TestShouldIgnoreSpecHashRemoveErrorReturnsFalseForPrivilegedPermissionError(t *testing.T) {
	// == setup ==
	containerSpec := minimalCreateSpec()
	err := &os.PathError{Op: "remove", Path: "/etc/raind/container/container-1/config_hash.json", Err: syscall.EACCES}

	// == exercise ==
	got := shouldIgnoreSpecHashRemoveError(containerSpec, err)

	// == assert ==
	assert.False(t, got)
}

func TestContainerInitLookEntrypointPathResolvesFromPATH(t *testing.T) {
	// == setup ==
	dir := t.TempDir()
	binPath := filepath.Join(dir, "app")
	require.NoError(t, os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0755))
	init := &ContainerInit{}

	// == exercise ==
	got, err := init.lookEntrypointPath("app", []string{"PATH=" + dir})

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, binPath, got)
}

func TestContainerInitLookEntrypointPathReturnsAbsolutePath(t *testing.T) {
	// == setup ==
	init := &ContainerInit{}

	// == exercise ==
	got, err := init.lookEntrypointPath("/bin/sh", nil)

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, "/bin/sh", got)
}

func TestRootContainerEnvPreparerSwitchesToNamespaceRoot(t *testing.T) {
	// == setup ==
	syscalls := &fakeInitSyscallHandler{}
	preparer := &rootContainerEnvPreparer{syscallHandler: syscalls}

	// == exercise ==
	err := preparer.switchToUserNamespaceRoot()

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, [][3]int{{0, 0, 0}}, syscalls.setresgidCalls)
	assert.Equal(t, [][3]int{{0, 0, 0}}, syscalls.setresuidCalls)
}

func TestRootContainerEnvPreparerSetsHostnameAndEnv(t *testing.T) {
	// == setup ==
	syscalls := &fakeInitSyscallHandler{}
	preparer := &rootContainerEnvPreparer{syscallHandler: syscalls}

	// == exercise ==
	require.NoError(t, preparer.setHostnameToContainerId("container-host"))
	require.NoError(t, preparer.setEnv([]string{"PATH=/bin", "HOME=/root"}))

	// == assert ==
	assert.Equal(t, []string{"container-host"}, syscalls.hostnames)
	assert.Equal(t, []string{"PATH=/bin", "HOME=/root"}, syscalls.envs)
}

func TestRootContainerEnvPreparerSetupOverlayBuildsMountData(t *testing.T) {
	// == setup ==
	syscalls := &fakeInitSyscallHandler{}
	preparer := &rootContainerEnvPreparer{syscallHandler: syscalls}
	imageAnnotation := `{"rootfsType":"overlay","imageLayer":["/l1","/l2"],"upperDir":"/upper","workDir":"/work"}`

	// == exercise ==
	err := preparer.setupOverlay("/rootfs", imageAnnotation)

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, []initMountCall{
		{source: "", target: "/", fstype: "", flags: syscall.MS_PRIVATE | syscall.MS_REC, data: ""},
		{source: "overlay", target: "/rootfs", fstype: "overlay", flags: 0, data: "lowerdir=/l1:/l2,upperdir=/upper,workdir=/work"},
		{source: "", target: "/rootfs", fstype: "", flags: syscall.MS_PRIVATE | syscall.MS_REC, data: ""},
	}, syscalls.mounts)
}

func TestRootContainerEnvPreparerSetupOverlaySeedsManagedFileTargets(t *testing.T) {
	// == setup ==
	syscalls := &fakeInitSyscallHandler{}
	preparer := &rootContainerEnvPreparer{syscallHandler: syscalls}
	imageAnnotation := `{"rootfsType":"overlay","imageLayer":["/l1"],"upperDir":"/upper","workDir":"/work"}`

	// == exercise ==
	err := preparer.setupOverlay("/rootfs", imageAnnotation)

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, []string{"/upper/etc", "/upper/etc", "/upper/etc"}, syscalls.mkdirAlls)
	assert.Equal(t, []string{
		"/upper/etc/resolv.conf",
		"/upper/etc/hostname",
		"/upper/etc/hosts",
	}, syscalls.openFiles)
}

func TestRootContainerEnvPreparerMountStdDeviceCreatesAndMountsDevices(t *testing.T) {
	// == setup ==
	syscalls := &fakeInitSyscallHandler{}
	preparer := &rootContainerEnvPreparer{syscallHandler: syscalls}

	// == exercise ==
	err := preparer.mountStdDevice("/rootfs")

	// == assert ==
	require.NoError(t, err)
	assert.Len(t, syscalls.creates, 6)
	assert.Len(t, syscalls.mounts, 12)
	assert.Equal(t, initMountCall{source: "/dev/random", target: "/rootfs/dev/random", fstype: "", flags: syscall.MS_BIND, data: ""}, syscalls.mounts[0])
	assert.Equal(t, initMountCall{source: "", target: "/rootfs/dev/random", fstype: "", flags: syscall.MS_BIND | syscall.MS_REMOUNT | syscall.MS_RDONLY | syscall.MS_NOEXEC | syscall.MS_NOSUID, data: ""}, syscalls.mounts[1])
}

func TestRootContainerEnvPreparerCreateSymbolicLinkCreatesMissingLinks(t *testing.T) {
	// == setup ==
	syscalls := &fakeInitSyscallHandler{}
	preparer := &rootContainerEnvPreparer{syscallHandler: syscalls}

	// == exercise ==
	err := preparer.createSymbolicLink("/rootfs")

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, [][2]string{
		{"/proc/self/fd", "/rootfs/dev/fd"},
		{"/proc/self/fd/0", "/rootfs/dev/stdin"},
		{"/proc/self/fd/1", "/rootfs/dev/stdout"},
		{"/proc/self/fd/2", "/rootfs/dev/stderr"},
		{"/dev/pts/ptmx", "/rootfs/dev/ptmx"},
	}, syscalls.symlinks)
}

func TestRootContainerEnvPreparerPivotRootRunsExpectedSyscalls(t *testing.T) {
	// == setup ==
	syscalls := &fakeInitSyscallHandler{}
	preparer := &rootContainerEnvPreparer{syscallHandler: syscalls}

	// == exercise ==
	err := preparer.pivotRoot("/rootfs")

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, []string{"/rootfs/put_old"}, syscalls.mkdirs)
	assert.Equal(t, [][2]string{{"/rootfs", "/rootfs/put_old"}}, syscalls.pivotRoots)
	assert.Equal(t, []string{"/"}, syscalls.chdirs)
	assert.Equal(t, []string{"/put_old"}, syscalls.unmounts)
	assert.Equal(t, []string{"/put_old"}, syscalls.rmdirs)
}
