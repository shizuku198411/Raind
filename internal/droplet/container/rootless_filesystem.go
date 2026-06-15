package container

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"
)

// prepareRootlessShiftedImageLayers creates per-container image lowerdir
// snapshots whose on-disk UID/GID ownership is shifted into the rootless user
// namespace mapping. The original image layer directories are shared runtime
// state and must not be chowned in place; changing them would break rootful
// containers and other rootless mappings. Instead, each lowerdir is copied into
// the container bundle and every filesystem object's owner is translated from
// container ID N to host ID base+N.
func prepareRootlessShiftedImageLayers(containerId string, containerSpec spec.Spec) (spec.Spec, error) {
	if !isRootlessSpec(containerSpec) {
		return containerSpec, nil
	}

	var imageConfig spec.ImageConfigObject
	if err := utils.StringToJson(containerSpec.Annotations.Image, &imageConfig); err != nil {
		return spec.Spec{}, err
	}
	if imageConfig.RootfsType != "overlay" || len(imageConfig.ImageLayer) == 0 {
		return containerSpec, nil
	}

	uidBase, gidBase, mapSize := rootlessIDMapConfig()
	snapshotRoot := rootlessShiftedLayerRoot(containerId)
	if err := os.RemoveAll(snapshotRoot); err != nil {
		return spec.Spec{}, fmt.Errorf("remove old rootless shifted layer root %q: %w", snapshotRoot, err)
	}
	if err := os.MkdirAll(snapshotRoot, 0o755); err != nil {
		return spec.Spec{}, fmt.Errorf("create rootless shifted layer root %q: %w", snapshotRoot, err)
	}

	shiftedLayers := make([]string, 0, len(imageConfig.ImageLayer))
	for i, layer := range imageConfig.ImageLayer {
		if layer == "" {
			continue
		}
		shiftedLayer := filepath.Join(snapshotRoot, fmt.Sprintf("layer-%d", i))
		if err := copyShiftedRootfsTree(layer, shiftedLayer, uidBase, gidBase, mapSize); err != nil {
			return spec.Spec{}, fmt.Errorf("create rootless shifted layer %q from %q: %w", shiftedLayer, layer, err)
		}
		shiftedLayers = append(shiftedLayers, shiftedLayer)
	}

	imageConfig.ImageLayer = shiftedLayers
	imageAnnotation, err := utils.JsonToString(imageConfig)
	if err != nil {
		return spec.Spec{}, err
	}
	containerSpec.Annotations.Image = imageAnnotation
	return containerSpec, nil
}

func rewriteContainerSpecAndHash(containerId string, containerSpec spec.Spec) error {
	configPath := utils.ConfigFilePath(containerId)
	if err := utils.WriteJsonToFile(configPath, containerSpec); err != nil {
		return err
	}

	hash, err := utils.Sha256File(configPath)
	if err != nil {
		return err
	}

	return utils.WriteJsonToFile(utils.ConfigFileHashPath(containerId), spec.SpecHash{Sha256: hash})
}

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

func rootlessShiftedLayerRoot(containerId string) string {
	return filepath.Join(utils.ContainerDir(containerId), "rootless-lower")
}

func rootlessIDMapConfig() (uidBase int, gidBase int, mapSize int) {
	return envInt("RAIND_ROOTLESS_UID_BASE", 100000), envInt("RAIND_ROOTLESS_GID_BASE", 100000), envInt("RAIND_ROOTLESS_ID_MAP_SIZE", 65536)
}

func copyShiftedRootfsTree(src string, dst string, uidBase int, gidBase int, mapSize int) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source rootfs %q is not a directory", src)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return copyShiftedPath(src, dst, uidBase, gidBase, mapSize)
}

func copyShiftedPath(src string, dst string, uidBase int, gidBase int, mapSize int) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("stat %q is not syscall.Stat_t", src)
	}
	shiftedUID, shiftedGID, err := shiftRootlessIDs(src, int(stat.Uid), int(stat.Gid), uidBase, gidBase, mapSize)
	if err != nil {
		return err
	}

	mode := info.Mode()
	switch {
	case mode.IsDir():
		if err := os.Mkdir(dst, copyModeBits(mode)); err != nil && !os.IsExist(err) {
			return err
		}
		if err := os.Lchown(dst, shiftedUID, shiftedGID); err != nil {
			return err
		}
		if err := os.Chmod(dst, copyModeBits(mode)); err != nil {
			return err
		}

		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyShiftedPath(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name()), uidBase, gidBase, mapSize); err != nil {
				return err
			}
		}
		return chtimesIfSupported(dst, info.ModTime())

	case mode.Type()&os.ModeSymlink != 0:
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		if err := os.Symlink(target, dst); err != nil {
			return err
		}
		return os.Lchown(dst, shiftedUID, shiftedGID)

	case mode.IsRegular():
		if err := copyRegularFile(src, dst, copyModeBits(mode)); err != nil {
			return err
		}
		if err := os.Lchown(dst, shiftedUID, shiftedGID); err != nil {
			return err
		}
		if err := os.Chmod(dst, copyModeBits(mode)); err != nil {
			return err
		}
		return chtimesIfSupported(dst, info.ModTime())

	case mode.Type()&(os.ModeDevice|os.ModeCharDevice|os.ModeNamedPipe) != 0:
		if err := syscall.Mknod(dst, uint32(stat.Mode), int(stat.Rdev)); err != nil {
			return err
		}
		return os.Lchown(dst, shiftedUID, shiftedGID)

	case mode.Type()&os.ModeSocket != 0:
		// Sockets cannot be meaningfully copied into an image snapshot. Image
		// layers should not contain live sockets, but ignore them if present.
		return nil

	default:
		return fmt.Errorf("unsupported rootfs file type: path=%s mode=%s", src, mode.String())
	}
}

func shiftRootlessIDs(path string, uid int, gid int, uidBase int, gidBase int, mapSize int) (int, int, error) {
	if uid < 0 || uid >= mapSize {
		return 0, 0, fmt.Errorf("uid outside rootless map: path=%s uid=%d map_size=%d", path, uid, mapSize)
	}
	if gid < 0 || gid >= mapSize {
		return 0, 0, fmt.Errorf("gid outside rootless map: path=%s gid=%d map_size=%d", path, gid, mapSize)
	}
	return uidBase + uid, gidBase + gid, nil
}

func copyModeBits(mode fs.FileMode) fs.FileMode {
	return mode & (os.ModePerm | os.ModeSetuid | os.ModeSetgid | os.ModeSticky)
}

func copyRegularFile(src string, dst string, perm fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

func chtimesIfSupported(path string, modTime time.Time) error {
	if err := os.Chtimes(path, modTime, modTime); err != nil && !os.IsPermission(err) {
		return err
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
