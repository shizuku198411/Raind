package container

import (
	"raind/internal/droplet/container/mountguard"
	"raind/internal/droplet/spec"
)

func securePath(rootfs, dest string) (string, error) {
	return mountguard.SecurePath(rootfs, dest)
}

func secureMount(source, target, fstype string, flags uintptr, data string, allowDevice bool) error {
	return mountguard.SecureMount(source, target, fstype, flags, data, allowDevice)
}

func hasDeniedSource(source string) bool {
	return mountguard.HasDeniedSource(source)
}

func hasDeniedDestination(destination string) bool {
	return mountguard.HasDeniedDestination(destination)
}

func isAllowedType(fstype string, options []string) bool {
	return mountguard.IsAllowedType(fstype, options)
}

func isAllowedKernelMountType(fstype string) bool {
	return mountguard.IsAllowedKernelMountType(fstype)
}

func isAllowedMountDataOption(option string) bool {
	return mountguard.IsAllowedMountDataOption(option)
}

func validateUserMount(m spec.MountObject) error {
	return mountguard.ValidateUserMount(m)
}

func isAllowedRuntimeVolumeSource(source string) bool {
	return mountguard.IsAllowedRuntimeVolumeSource(source)
}

func hasMountOption(options []string, want string) bool {
	return mountguard.HasMountOption(options, want)
}

func isUnderPath(path string, base string) bool {
	return mountguard.IsUnderPath(path, base)
}

func isSymlink(source string) (bool, error) {
	return mountguard.IsSymlink(source)
}

type WalkLimits = mountguard.WalkLimits

func rejectSymlinkInDirTreeFd(root string, lim WalkLimits) error {
	return mountguard.RejectSymlinkInDirTreeFd(root, lim)
}
