package container

import (
	"raind/internal/droplet/container/bundle"
)

const ociConfigFileName = bundle.OCIConfigFileName

func bundlePathForContainer(containerId string, bundlePath string) (string, error) {
	return bundle.PathForContainer(containerId, bundlePath)
}

func prepareBundleConfig(containerId string, bundlePath string) error {
	return bundle.PrepareConfig(containerId, bundlePath)
}

func resolveBundleRootPath(bundlePath string, rootPath string) string {
	return bundle.ResolveRootPath(bundlePath, rootPath)
}

func writeContainerPidFile(pidFile string, pid int) error {
	return bundle.WriteContainerPidFile(pidFile, pid)
}

func writeExternalPidFileMarker(containerId string, pidFile string) error {
	return bundle.WriteExternalPidFileMarker(containerId, pidFile)
}

func writeSpecHashFile(containerId string) error {
	return writeCurrentSpecHash(containerId)
}
