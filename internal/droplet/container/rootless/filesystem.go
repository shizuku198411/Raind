package rootless

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"raind/internal/droplet/container/spechash"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"
)

var Lchown = os.Lchown

// prepareRootlessShiftedImageLayers rewrites overlay lowerdirs to rootless
// shifted layer caches whose on-disk UID/GID ownership is translated into the
// rootless user namespace mapping. The original image layer directories are
// shared runtime state and must not be chowned in place; changing them would
// break rootful containers and other rootless mappings. Instead, each lowerdir
// is cached under the image bundle and every filesystem object's owner is
// translated according to the rootless ID mapping policy on first rootless use.
func PrepareShiftedImageLayers(containerId string, containerSpec spec.Spec, rootlessConfigs ...spec.RootlessConfigObject) (spec.Spec, error) {
	rootlessConfig := spec.RootlessConfigObject{}
	if len(rootlessConfigs) > 0 {
		rootlessConfig = rootlessConfigs[0]
	} else if cfg, ok := ConfigFromSpec(containerSpec); ok {
		rootlessConfig = cfg
	}
	if !rootlessConfig.Enabled {
		return containerSpec, nil
	}

	var imageConfig spec.ImageConfigObject
	if err := utils.StringToJson(containerSpec.Annotations.Image, &imageConfig); err != nil {
		return spec.Spec{}, err
	}
	if imageConfig.RootfsType != "overlay" || len(imageConfig.ImageLayer) == 0 {
		return containerSpec, nil
	}

	policy := IDMapPolicyFromConfig(rootlessConfig)
	shiftedLayers := make([]string, 0, len(imageConfig.ImageLayer))
	for _, layer := range imageConfig.ImageLayer {
		if layer == "" {
			continue
		}
		shiftedLayer, err := PrepareShiftedLayerCache(layer, policy)
		if err != nil {
			return spec.Spec{}, fmt.Errorf("prepare rootless shifted layer cache for %q: %w", layer, err)
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

func RewriteContainerSpecAndHash(containerId string, containerSpec spec.Spec) error {
	configPath := utils.ConfigFilePath(containerId)
	if err := utils.WriteJsonToFile(configPath, containerSpec); err != nil {
		return err
	}

	return spechash.WriteCurrent(containerId)
}

// prepareRootlessWritableFilesystem makes host-owned container filesystem
// paths writable by the init process after it enters the rootless user
// namespace. Namespace root maps to the selected host UID/GID, so the overlay
// upper/work directories and rootfs mount target must be owned by that host
// identity before the init process attempts overlay setup.
func PrepareWritableFilesystem(containerSpec spec.Spec) error {
	rootlessConfig, ok := ConfigFromSpec(containerSpec)
	if !ok {
		return nil
	}
	if CurrentUserNamespaceDiffersFromInit() {
		return nil
	}

	var imageConfig spec.ImageConfigObject
	if err := utils.StringToJson(containerSpec.Annotations.Image, &imageConfig); err != nil {
		return err
	}

	uid, gid := HostRootID(rootlessConfig)
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

func PrepareShiftedLayerCache(src string, policy IDMapPolicy) (string, error) {
	if ShiftedLayerCacheReady(src, ShiftedLayerCompleteMarker(filepath.Dir(src))) && isRootlessShiftedLayerPath(src) {
		return src, nil
	}
	cacheRoot, err := rootlessShiftedLayerCacheRootForPolicy(src, policy)
	if err != nil {
		return "", err
	}
	cacheRootfs := filepath.Join(cacheRoot, "rootfs")
	completeMarker := ShiftedLayerCompleteMarker(cacheRoot)
	if ShiftedLayerCacheReady(cacheRootfs, completeMarker) {
		return cacheRootfs, nil
	}

	lockDir := cacheRoot + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockDir), 0o755); err != nil {
		return "", fmt.Errorf("create rootless shifted layer cache parent %q: %w", filepath.Dir(lockDir), err)
	}
	lockTaken, err := tryCreateRootlessCacheLock(lockDir)
	if err != nil {
		return "", err
	}
	if !lockTaken {
		if err := waitRootlessShiftedLayerCache(cacheRootfs, completeMarker, 60*time.Second); err != nil {
			return "", err
		}
		return cacheRootfs, nil
	}
	defer os.RemoveAll(lockDir)

	// Another creator may have completed the cache before this process acquired
	// the lock. Re-check while holding the lock directory.
	if ShiftedLayerCacheReady(cacheRootfs, completeMarker) {
		return cacheRootfs, nil
	}

	tmpRoot := fmt.Sprintf("%s.tmp.%d", cacheRoot, os.Getpid())
	if err := os.RemoveAll(tmpRoot); err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpRoot)

	tmpRootfs := filepath.Join(tmpRoot, "rootfs")
	if err := copyShiftedRootfsTree(src, tmpRootfs, policy); err != nil {
		return "", err
	}
	if err := os.WriteFile(ShiftedLayerCompleteMarker(tmpRoot), []byte("complete\n"), 0o644); err != nil {
		return "", err
	}

	if err := os.RemoveAll(cacheRoot); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(cacheRoot), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(tmpRoot, cacheRoot); err != nil {
		return "", err
	}
	return cacheRootfs, nil
}

func isRootlessShiftedLayerPath(src string) bool {
	cleanSrc := filepath.Clean(src)
	return filepath.Base(cleanSrc) == "rootfs" && filepath.Base(filepath.Dir(filepath.Dir(cleanSrc))) == "rootless-shifted"
}

func waitRootlessShiftedLayerCache(rootfsPath string, completeMarker string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ShiftedLayerCacheReady(rootfsPath, completeMarker) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for rootless shifted layer cache: %s", filepath.Dir(rootfsPath))
}

func ShiftedLayerCacheRoot(src string, uidBase int, gidBase int, mapSize int) (string, error) {
	return rootlessShiftedLayerCacheRootForPolicy(src, IDMapPolicy{
		mode:    spec.RootlessModeShiftedRoot,
		uidBase: uidBase,
		gidBase: gidBase,
		mapSize: mapSize,
	})
}

func rootlessShiftedLayerCacheRootForPolicy(src string, policy IDMapPolicy) (string, error) {
	if src == "" {
		return "", fmt.Errorf("empty rootfs layer path")
	}
	cleanSrc := filepath.Clean(src)
	if filepath.Base(cleanSrc) != "rootfs" {
		return "", fmt.Errorf("rootless shifted cache requires layer rootfs path, got %q", src)
	}
	bundleDir := filepath.Dir(cleanSrc)
	return filepath.Join(bundleDir, "rootless-shifted", ShiftedLayerCacheKey(policy)), nil
}

func ShiftedLayerCacheKey(policy IDMapPolicy) string {
	if policy.mode == spec.RootlessModeLoginRoot {
		return fmt.Sprintf("mode_%s_rootuid_%d_rootgid_%d_uid_%d_gid_%d_size_%d_v1", policy.mode, policy.rootUID, policy.rootGID, policy.uidBase, policy.gidBase, policy.mapSize)
	}
	return fmt.Sprintf("uid_%d_gid_%d_size_%d_v1", policy.uidBase, policy.gidBase, policy.mapSize)
}

func ShiftedLayerCompleteMarker(cacheRoot string) string {
	return filepath.Join(cacheRoot, ".raind-rootless-shift-complete")
}

func ShiftedLayerCacheReady(rootfsPath string, completeMarker string) bool {
	rootfsInfo, rootfsErr := os.Stat(rootfsPath)
	if rootfsErr != nil || !rootfsInfo.IsDir() {
		return false
	}
	_, markerErr := os.Stat(completeMarker)
	return markerErr == nil
}

func tryCreateRootlessCacheLock(lockDir string) (bool, error) {
	if err := os.Mkdir(lockDir, 0o755); err != nil {
		if os.IsExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func rootlessIDMapConfig() (uidBase int, gidBase int, mapSize int) {
	return envInt("RAIND_ROOTLESS_UID_BASE", 100000), envInt("RAIND_ROOTLESS_GID_BASE", 100000), envInt("RAIND_ROOTLESS_ID_MAP_SIZE", 65536)
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func copyShiftedRootfsTree(src string, dst string, policy IDMapPolicy) error {
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
	return CopyShiftedPath(src, dst, policy)
}

func CopyShiftedPath(src string, dst string, policy IDMapPolicy) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}

	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("stat %q is not syscall.Stat_t", src)
	}
	shiftedUID, shiftedGID, err := ShiftIDs(src, int(stat.Uid), int(stat.Gid), policy)
	if err != nil {
		return err
	}

	mode := info.Mode()
	switch {
	case mode.IsDir():
		if err := os.Mkdir(dst, copyModeBits(mode)); err != nil && !os.IsExist(err) {
			return err
		}
		if err := Lchown(dst, shiftedUID, shiftedGID); err != nil {
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
			if err := CopyShiftedPath(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name()), policy); err != nil {
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
		return Lchown(dst, shiftedUID, shiftedGID)

	case mode.IsRegular():
		if err := copyRegularFile(src, dst, copyModeBits(mode)); err != nil {
			return err
		}
		if err := Lchown(dst, shiftedUID, shiftedGID); err != nil {
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
		return Lchown(dst, shiftedUID, shiftedGID)

	case mode.Type()&os.ModeSocket != 0:
		// Sockets cannot be meaningfully copied into an image snapshot. Image
		// layers should not contain live sockets, but ignore them if present.
		return nil

	default:
		return fmt.Errorf("unsupported rootfs file type: path=%s mode=%s", src, mode.String())
	}
}

func ShiftIDs(path string, uid int, gid int, args ...any) (int, int, error) {
	policy, err := rootlessIDMapPolicyFromShiftArgs(args...)
	if err != nil {
		return 0, 0, err
	}
	shiftedUID, err := policy.MapUID(path, uid)
	if err != nil {
		return 0, 0, err
	}
	shiftedGID, err := policy.MapGID(path, gid)
	if err != nil {
		return 0, 0, err
	}
	return shiftedUID, shiftedGID, nil
}

func rootlessIDMapPolicyFromShiftArgs(args ...any) (IDMapPolicy, error) {
	if len(args) == 1 {
		policy, ok := args[0].(IDMapPolicy)
		if !ok {
			return IDMapPolicy{}, fmt.Errorf("invalid rootless ID map policy argument")
		}
		return policy, nil
	}
	if len(args) == 3 {
		uidBase, uidOK := args[0].(int)
		gidBase, gidOK := args[1].(int)
		mapSize, sizeOK := args[2].(int)
		if !uidOK || !gidOK || !sizeOK {
			return IDMapPolicy{}, fmt.Errorf("invalid rootless ID map legacy arguments")
		}
		return IDMapPolicy{mode: spec.RootlessModeShiftedRoot, uidBase: uidBase, gidBase: gidBase, mapSize: mapSize}, nil
	}
	return IDMapPolicy{}, fmt.Errorf("invalid rootless ID map arguments")
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
		if err := Lchown(path, uid, gid); err != nil {
			return err
		}
		return nil
	})
}
