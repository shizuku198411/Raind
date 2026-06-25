package container

import (
	"fmt"
	"os"
	"path/filepath"
	"raind/internal/droplet/spec"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// mount path validation for protecting path traversal
func securePath(rootfs, dest string) (string, error) {
	rel := strings.TrimPrefix(dest, "/")
	clean := filepath.Clean(rel)

	if clean == "." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) || clean == ".." {
		return "", fmt.Errorf("invalid destination: %q", dest)
	}

	fullPath := filepath.Join(rootfs, clean)
	root := filepath.Clean(rootfs) + string(os.PathSeparator)
	if !strings.HasPrefix(fullPath+string(os.PathSeparator), root) && fullPath != filepath.Clean(rootfs) {
		return "", fmt.Errorf("destination escapes rootfs: %q -> %q", dest, fullPath)
	}
	return fullPath, nil
}

func secureMount(source, target, fstype string, flags uintptr, data string, allowDevice bool) error {
	if fstype == "bind" {
		// bind mount should use MS_BIND with empty fstype
		fstype = ""
	}
	// 1. bind mount
	if err := syscall.Mount(source, target, fstype, flags, data); err != nil {
		return err
	}
	// force nosuid/nodev/noexec on bind mount
	if fstype == "bind" || flags&syscall.MS_BIND != 0 {
		remountFlags := syscall.MS_BIND | syscall.MS_REMOUNT | syscall.MS_NOEXEC | syscall.MS_NOSUID
		if !allowDevice {
			remountFlags |= syscall.MS_NODEV
		}
		if flags&syscall.MS_RDONLY != 0 {
			remountFlags |= syscall.MS_RDONLY
		}
		if err := syscall.Mount("", target, "", uintptr(remountFlags), ""); err != nil {
			return err
		}
	}

	// 2. protect mount propagation
	if err := syscall.Mount("", target, "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return err
	}
	return nil
}

// source mount validation
// the following source is denied by default
//
//	/proc, /sys, /dev, /run, /var/run, /boot, /root, /, /bin, /usr/bin, /usr/local/bin
func hasDeniedSource(source string) bool {
	p := filepath.Clean(source)
	if p == "/" {
		return true
	}
	deniedList := []string{"/proc", "/sys", "/dev", "/run", "/var/run", "/boot", "/root", "/bin", "/usr/bin", "/usr/local/bin", "/etc/raind"}
	for _, d := range deniedList {
		if p == d || strings.HasPrefix(p, d+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func hasDeniedDestination(destination string) bool {
	p := filepath.Clean(destination)
	if p == "/" {
		return true
	}
	deniedList := []string{"/proc", "/sys", "/dev", "/run", "/var/run", "/boot"}
	for _, d := range deniedList {
		if p == d || strings.HasPrefix(p, d+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func isAllowedType(fstype string, options []string) bool {
	kernelMount := isAllowedKernelMountType(fstype)
	if fstype != "" && fstype != "bind" && !kernelMount {
		return false
	}

	hasBind := fstype == "bind"
	for _, o := range options {
		switch o {
		case "bind", "rbind":
			if kernelMount {
				return false
			}
			hasBind = true
		case "rprivate", "private", "ro", "rw", "nosuid", "nodev", "noexec", "relatime", "noatime", "strictatime", "dev", "newinstance":
			// allowed mount option
		case "z", "Z":
			// Docker-compatible SELinux relabel hints. Raind does not relabel, but accepting them
			// keeps common volume declarations from failing before the bind mount is made.
		default:
			if !kernelMount || !isAllowedMountDataOption(o) {
				return false
			}
		}
	}
	return kernelMount || hasBind || len(options) == 0
}

func isAllowedKernelMountType(fstype string) bool {
	switch fstype {
	case "proc", "sysfs", "tmpfs", "devpts", "cgroup2", "mqueue":
		return true
	default:
		return false
	}
}

func isAllowedMountDataOption(option string) bool {
	key, _, ok := strings.Cut(option, "=")
	if !ok {
		return false
	}
	switch key {
	case "mode", "size", "gid", "uid", "ptmxmode":
		return true
	default:
		return false
	}
}

func validateUserMount(m spec.MountObject) error {
	if !isAllowedType(m.Type, m.Options) {
		return fmt.Errorf("invalid mount type/options: type=%q options=%q", m.Type, strings.Join(m.Options, ","))
	}
	if isAllowedKernelMountType(m.Type) {
		return nil
	}

	allowDevice := hasMountOption(m.Options, "dev")
	if hasDeniedSource(m.Source) && !(allowDevice && isUnderPath(m.Source, "/dev")) && !isAllowedRuntimeVolumeSource(m.Source) {
		return fmt.Errorf("invalid mount source: %s", m.Source)
	}
	if hasDeniedDestination(m.Destination) && !(allowDevice && isUnderPath(m.Destination, "/dev")) {
		return fmt.Errorf("invalid mount destination: %s", m.Destination)
	}
	return nil
}

func isAllowedRuntimeVolumeSource(source string) bool {
	return isUnderPath(source, "/etc/raind/volume/pvc")
}

func hasMountOption(options []string, want string) bool {
	for _, o := range options {
		if o == want {
			return true
		}
	}
	return false
}

func isUnderPath(path string, base string) bool {
	p := filepath.Clean(path)
	b := filepath.Clean(base)
	return p == b || strings.HasPrefix(p, b+string(os.PathSeparator))
}

func isSymlink(source string) (bool, error) {
	fi, err := os.Lstat(source)
	if err != nil {
		return false, err
	}
	return fi.Mode()&os.ModeSymlink != 0, nil
}

type WalkLimits struct {
	MaxDepth   int
	MaxEntries int
}

func rejectSymlinkInDirTreeFd(root string, lim WalkLimits) error {
	if lim.MaxDepth <= 0 {
		lim.MaxDepth = 64
	}
	if lim.MaxEntries <= 0 {
		lim.MaxEntries = 200_000
	}

	// Root: reject symlink and open as directory without following symlinks.
	rootFd, err := unix.Open(root, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("open root failed: %s: %w", root, err)
	}
	defer unix.Close(rootFd)

	var st unix.Stat_t
	if err := unix.Fstat(rootFd, &st); err != nil {
		return fmt.Errorf("fstat root failed: %s: %w", root, err)
	}
	modeType := st.Mode & unix.S_IFMT
	if modeType == unix.S_IFLNK {
		return fmt.Errorf("source:%s is symlink", root)
	}
	if modeType != unix.S_IFDIR {
		// root is a regular file/device etc (not a directory). We only care that it's not a symlink.
		return nil
	}

	// Re-open as directory FD (O_DIRECTORY) for readdir.
	unix.Close(rootFd)
	rootFd, err = unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("open root dir failed: %s: %w", root, err)
	}
	defer unix.Close(rootFd)

	entries := 0

	var walk func(dirfd int, absPath string, depth int) error
	walk = func(dirfd int, absPath string, depth int) error {
		if depth > lim.MaxDepth {
			return fmt.Errorf("mount source tree too deep: %s (depth>%d)", absPath, lim.MaxDepth)
		}

		// Read directory entries from dirfd
		buf := make([]byte, 32*1024)
		for {
			n, err := unix.ReadDirent(dirfd, buf)
			if err != nil {
				return fmt.Errorf("readdir failed: %s: %w", absPath, err)
			}
			if n == 0 {
				return nil
			}

			// Parse entry names. d_type is not used; we will fstatat each entry.
			names := make([]string, 0, 128)
			_, _, names = unix.ParseDirent(buf[:n], -1, names)

			for _, name := range names {
				if name == "." || name == ".." {
					continue
				}

				entries++
				if entries > lim.MaxEntries {
					return fmt.Errorf("mount source tree too large: %s (entries>%d)", absPath, lim.MaxEntries)
				}

				var st unix.Stat_t
				if err := unix.Fstatat(dirfd, name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
					return fmt.Errorf("fstatat failed: %s/%s: %w", absPath, name, err)
				}

				t := st.Mode & unix.S_IFMT
				if t == unix.S_IFLNK {
					return fmt.Errorf("symlink found under mount source: %s", filepath.Join(absPath, name))
				}

				if t == unix.S_IFDIR {
					// open child dir without following symlinks
					childFd, err := unix.Openat(dirfd, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
					if err != nil {
						return fmt.Errorf("openat dir failed: %s/%s: %w", absPath, name, err)
					}
					err = walk(childFd, filepath.Join(absPath, name), depth+1)
					unix.Close(childFd)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return walk(rootFd, filepath.Clean(root), 0)
}
