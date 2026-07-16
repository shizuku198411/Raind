package container

import (
	"os"

	"raind/internal/droplet/container/rootless"
	"raind/internal/droplet/spec"
)

var rootlessLchown = os.Lchown

func prepareRootlessShiftedImageLayers(containerId string, containerSpec spec.Spec, rootlessConfigs ...spec.RootlessConfigObject) (spec.Spec, error) {
	syncRootlessLchown()
	return rootless.PrepareShiftedImageLayers(containerId, containerSpec, rootlessConfigs...)
}

func rewriteContainerSpecAndHash(containerId string, containerSpec spec.Spec) error {
	return rootless.RewriteContainerSpecAndHash(containerId, containerSpec)
}

func prepareRootlessWritableFilesystem(containerSpec spec.Spec) error {
	syncRootlessLchown()
	return rootless.PrepareWritableFilesystem(containerSpec)
}

func prepareRootlessShiftedLayerCache(src string, policy rootlessIDMapPolicy) (string, error) {
	syncRootlessLchown()
	return rootless.PrepareShiftedLayerCache(src, policy.toRootless())
}

func rootlessShiftedLayerCacheRoot(src string, uidBase int, gidBase int, mapSize int) (string, error) {
	return rootless.ShiftedLayerCacheRoot(src, uidBase, gidBase, mapSize)
}

func rootlessShiftedLayerCacheKey(policy rootlessIDMapPolicy) string {
	return rootless.ShiftedLayerCacheKey(policy.toRootless())
}

func rootlessShiftedLayerCompleteMarker(cacheRoot string) string {
	return rootless.ShiftedLayerCompleteMarker(cacheRoot)
}

func rootlessShiftedLayerCacheReady(rootfsPath string, completeMarker string) bool {
	return rootless.ShiftedLayerCacheReady(rootfsPath, completeMarker)
}

func shiftRootlessIDs(path string, uid int, gid int, args ...any) (int, int, error) {
	shiftArgs := make([]any, 0, len(args))
	for _, arg := range args {
		if policy, ok := arg.(rootlessIDMapPolicy); ok {
			shiftArgs = append(shiftArgs, policy.toRootless())
			continue
		}
		shiftArgs = append(shiftArgs, arg)
	}
	return rootless.ShiftIDs(path, uid, gid, shiftArgs...)
}

func copyShiftedPath(src string, dst string, policy rootlessIDMapPolicy) error {
	syncRootlessLchown()
	return rootless.CopyShiftedPath(src, dst, policy.toRootless())
}

func syncRootlessLchown() {
	rootless.Lchown = rootlessLchown
}
