package container

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"raind/internal/condenser/core/image"
)

func reportRootlessShiftedLayerCacheProgress(createParameter ServiceCreateModel, imageLayer string) {
	if !createParameter.Rootless || createParameter.Progress == nil || imageLayer == "" {
		return
	}

	uidBase, gidBase, mapSize := rootlessCacheIDMapConfig()
	cacheRoot, err := rootlessShiftedLayerCacheRoot(imageLayer, uidBase, gidBase, mapSize)
	if err != nil {
		return
	}

	detail := "creating rootless shifted layer cache"
	if rootlessShiftedLayerCacheReady(filepath.Join(cacheRoot, "rootfs"), rootlessShiftedLayerCompleteMarker(cacheRoot)) {
		detail = "rootless shifted layer cache found"
	}

	createParameter.Progress(image.PullProgressEvent{
		Status: "rootless-cache",
		ID:     createParameter.Image,
		Detail: detail,
	})
}

func rootlessCacheIDMapConfig() (uidBase int, gidBase int, mapSize int) {
	return envInt("RAIND_ROOTLESS_UID_BASE", 100000), envInt("RAIND_ROOTLESS_GID_BASE", 100000), envInt("RAIND_ROOTLESS_ID_MAP_SIZE", 65536)
}

func rootlessShiftedLayerCacheRoot(src string, uidBase int, gidBase int, mapSize int) (string, error) {
	if src == "" {
		return "", fmt.Errorf("empty rootfs layer path")
	}
	cleanSrc := filepath.Clean(src)
	if filepath.Base(cleanSrc) != "rootfs" {
		return "", fmt.Errorf("rootless shifted cache requires layer rootfs path, got %q", src)
	}
	bundleDir := filepath.Dir(cleanSrc)
	return filepath.Join(bundleDir, "rootless-shifted", rootlessShiftedLayerCacheKey(uidBase, gidBase, mapSize)), nil
}

func rootlessShiftedLayerCacheKey(uidBase int, gidBase int, mapSize int) string {
	return fmt.Sprintf("uid_%d_gid_%d_size_%d_v1", uidBase, gidBase, mapSize)
}

func rootlessShiftedLayerCompleteMarker(cacheRoot string) string {
	return filepath.Join(cacheRoot, ".raind-rootless-shift-complete")
}

func rootlessShiftedLayerCacheReady(rootfsPath string, completeMarker string) bool {
	rootfsInfo, rootfsErr := os.Stat(rootfsPath)
	if rootfsErr != nil || !rootfsInfo.IsDir() {
		return false
	}
	_, markerErr := os.Stat(completeMarker)
	return markerErr == nil
}

func envInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}
