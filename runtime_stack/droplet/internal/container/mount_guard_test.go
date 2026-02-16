package container

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecurePath_Success(t *testing.T) {
	// == arrange ==
	rootfs := "/etc/raind/container/111111/merged"
	dest := "/inside/container"

	// == act ==
	path, err := securePath(rootfs, dest)

	// == assert ==
	assert.Nil(t, err)
	assert.Equal(t, "/etc/raind/container/111111/merged/inside/container", path)
}

func TestSecurePath_IncludeRelativePathError(t *testing.T) {
	// == arrange ==
	rootfs := "/etc/raind/container/111111/merged"
	dest := "/../container"

	// == act ==
	_, err := securePath(rootfs, dest)

	// == assert ==
	assert.NotNil(t, err)
}

func TestHasDeniedSource_NoDeniedSource(t *testing.T) {
	// == arrange ==
	source := "/home/user/path"

	// == act ==
	result := hasDeniedSource(source)

	// == assert ==
	assert.False(t, result)
}

func TestHasDeniedSource_HasDeniedSource(t *testing.T) {
	// == arrange ==
	source_1 := "/root/path"
	source_2 := "/proc/self"
	source_3 := "/sys/fs"
	source_4 := "/dev/pts"
	source_5 := "/run/user"
	source_6 := "/var/run/user"
	source_7 := "/boot/firmware"

	// == act ==
	res_1 := hasDeniedSource(source_1)
	res_2 := hasDeniedSource(source_2)
	res_3 := hasDeniedSource(source_3)
	res_4 := hasDeniedSource(source_4)
	res_5 := hasDeniedSource(source_5)
	res_6 := hasDeniedSource(source_6)
	res_7 := hasDeniedSource(source_7)

	// == assert ==
	assert.True(t, res_1)
	assert.True(t, res_2)
	assert.True(t, res_3)
	assert.True(t, res_4)
	assert.True(t, res_5)
	assert.True(t, res_6)
	assert.True(t, res_7)
}

func TestHasDeniedDestination_NoDeniedDest(t *testing.T) {
	// == arrange ==
	dest := "/home/user/path"

	// == act ==
	result := hasDeniedDestination(dest)

	// == assert ==
	assert.False(t, result)
}

func TestHasDeniedSource_HasDeniedDest(t *testing.T) {
	// == arrange ==
	source_1 := "/proc/self"
	source_2 := "/sys/fs"
	source_3 := "/dev/pts"
	source_4 := "/run/user"
	source_5 := "/var/run/user"
	source_6 := "/boot/firmware"

	// == act ==
	res_1 := hasDeniedDestination(source_1)
	res_2 := hasDeniedDestination(source_2)
	res_3 := hasDeniedDestination(source_3)
	res_4 := hasDeniedDestination(source_4)
	res_5 := hasDeniedDestination(source_5)
	res_6 := hasDeniedDestination(source_6)

	// == assert ==
	assert.True(t, res_1)
	assert.True(t, res_2)
	assert.True(t, res_3)
	assert.True(t, res_4)
	assert.True(t, res_5)
	assert.True(t, res_6)
}

func TestIsSymlink_NotSymlink(t *testing.T) {
	// == arrange ==
	tmp := t.TempDir()
	// file
	file := filepath.Join(tmp, "test.txt")
	fd, _ := os.OpenFile(file, os.O_WRONLY|os.O_CREATE, 0644)
	fd.Close()
	// directory
	dir := filepath.Join(tmp, "test")
	_ = os.Mkdir(dir, 0755)

	// == act ==
	res_1, err_1 := isSymlink(file)
	res_2, err_2 := isSymlink(dir)

	// == assert ==
	assert.Nil(t, err_1)
	assert.False(t, res_1)
	assert.Nil(t, err_2)
	assert.False(t, res_2)
}

func TestIsSymlink_IsSymlink(t *testing.T) {
	// == arrange ==
	tmp := t.TempDir()
	// symlink
	symlink := filepath.Join(tmp, "symlink")
	_ = os.Symlink("/dummy", symlink)

	// == act ==
	res, err := isSymlink(symlink)

	// == assert ==
	assert.Nil(t, err)
	assert.True(t, res)
}

func TestRejectSymlinkInDirTreeFd_NoSymlinkExist(t *testing.T) {
	// == arrange ==
	tmp := t.TempDir()
	// create dir
	dirpath := filepath.Join(tmp, "dir1", "dir2")
	_ = os.MkdirAll(dirpath, 0755)
	// create file
	filepath_1 := filepath.Join(tmp, "dir1", "test.txt")
	filepath_2 := filepath.Join(tmp, "dir1", "dir2", "test.txt")
	_, _ = os.Create(filepath_1)
	_, _ = os.Create(filepath_2)

	// == act ==
	err := rejectSymlinkInDirTreeFd(tmp, WalkLimits{})

	// == assert ==
	assert.Nil(t, err)
}

func TestRejectSymlinkInDirTreeFd_SymlinkExist(t *testing.T) {
	// == arrange ==
	tmp := t.TempDir()
	// create dir
	dirpath := filepath.Join(tmp, "dir1", "dir2")
	_ = os.MkdirAll(dirpath, 0755)
	// create file
	filepath_1 := filepath.Join(tmp, "dir1", "test.txt")
	filepath_2 := filepath.Join(tmp, "dir1", "dir2", "test.txt")
	symlink := filepath.Join(tmp, "dir1", "dir2", "symlink")
	_, _ = os.Create(filepath_1)
	_, _ = os.Create(filepath_2)
	_ = os.Symlink("/dummy", symlink)

	// == act ==
	err := rejectSymlinkInDirTreeFd(tmp, WalkLimits{})

	// == assert ==
	assert.NotNil(t, err)
}

func TestRejectSymlinkInDirTreeFd_MaxDepth(t *testing.T) {
	// == arrange ==
	tmp := t.TempDir()
	// create dir
	dirpath := filepath.Join(tmp, "dir1", "dir2", "dir3", "dir4")
	_ = os.MkdirAll(dirpath, 0755)
	// create file
	filepath_1 := filepath.Join(dirpath, "test.txt")
	_, _ = os.Create(filepath_1)

	// == act ==
	err := rejectSymlinkInDirTreeFd(tmp, WalkLimits{MaxDepth: 3})

	// == assert ==
	assert.NotNil(t, err)
}

func TestRejectSymlinkInDirTreeFd_MaxEntries(t *testing.T) {
	// == arrange ==
	tmp := t.TempDir()
	// create dir
	dirpath := filepath.Join(tmp, "dir1", "dir2", "dir3", "dir4")
	_ = os.MkdirAll(dirpath, 0755)
	// create file
	filepath_1 := filepath.Join(dirpath, "test.txt")
	filepath_2 := filepath.Join(dirpath, "test2.txt")
	_, _ = os.Create(filepath_1)
	_, _ = os.Create(filepath_2)

	// == act ==
	err := rejectSymlinkInDirTreeFd(tmp, WalkLimits{MaxEntries: 1})

	// == assert ==
	assert.NotNil(t, err)
}

func TestIsAllowedType_FstypeBind(t *testing.T) {
	// == arrange ==
	fstype := "bind"
	options := []string{"rbind", "rprivate"}

	// == act ==
	result := isAllowedType(fstype, options)

	// == assert ==
	assert.True(t, result)
}

func TestIsAllowedType_OptionBind(t *testing.T) {
	// == arrange ==
	fstype := ""
	options := []string{"bind"}

	// == act ==
	result := isAllowedType(fstype, options)

	// == assert ==
	assert.True(t, result)
}

func TestIsAllowedType_NotAllowedFstype(t *testing.T) {
	// == arrange ==
	fstype := "sysfs"
	options := []string{"bind"}

	// == act ==
	result := isAllowedType(fstype, options)

	// == assert ==
	assert.False(t, result)
}

func TestIsAllowedType_NotAllowedOptions_1(t *testing.T) {
	// == arrange ==
	fstype := "bind"
	options := []string{"rbind", "rprivate", "gid=5"}

	// == act ==
	result := isAllowedType(fstype, options)

	// == asset ==
	assert.False(t, result)
}

func TestIsAllowedType_NotAllowedOptions_2(t *testing.T) {
	// == arrange ==
	fstype := "bind"
	options := []string{"rbind"}

	// == act ==
	result := isAllowedType(fstype, options)

	// == asset ==
	assert.False(t, result)
}
