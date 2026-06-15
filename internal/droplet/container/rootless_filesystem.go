package container

import (
	"fmt"
	"os"
	"path/filepath"

	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"
)

// prepareRootlessWritableFilesystem makes host-owned container filesystem
// paths writable by the init process after it enters the rootless user
// namespace. Namespace root maps to the configured host UID/GID range, so the
// overlay upper/work directories and rootfs mount target must be owned by that
// host identity before the init process attempts overlay setup.
func prepareRootlessWritableFilesystem(containerSpec spec.Spec) error {
	if !isRootlessSpec(containerSpec) {
		return nil
	}

	var imageConfig spec.ImageConfigObject
	if err := utils.StringToJson(containerSpec.Annotations.Image, &imageConfig); err != nil {
		return err
	}

	uid, gid := rootlessHostRootID()
	for _, path := range []string{imageConfig.UpperDir, imageConfig.WorkDir, containerSpec.Root.Path} {
		if path == "" {
			continue
		}
		if err := chownPathTree(path, uid, gid); err != nil {
			return fmt.Errorf("chown rootless writable path %q to %d:%d: %w", path, uid, gid, err)
		}
	}

	return nil
}

func chownPathTree(root string, uid int, gid int) error {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}

	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := os.Lchown(path, uid, gid); err != nil {
			return err
		}
		return nil
	})
}
