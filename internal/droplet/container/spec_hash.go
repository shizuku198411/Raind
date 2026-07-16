package container

import (
	"raind/internal/droplet/container/spechash"
	"raind/internal/droplet/spec"
)

func sealAndLoadSpec(containerId string, loader specLoader) (spec.Spec, error) {
	return spechash.SealAndLoad(containerId, loader.loadFile)
}

func verifyAndLoadSpec(containerId string, loader specLoader) (spec.Spec, error) {
	return spechash.VerifyAndLoad(containerId, loader.loadFile)
}

func verifyLoadAndConsumeSpecHash(containerId string, loader specLoader) (spec.Spec, error) {
	return spechash.VerifyLoadAndConsume(containerId, loader.loadFile, shouldIgnoreSpecHashRemoveError)
}

func writeCurrentSpecHash(containerId string) error {
	return spechash.WriteCurrent(containerId)
}
