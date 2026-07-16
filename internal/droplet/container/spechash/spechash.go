package spechash

import (
	"fmt"
	"os"

	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"
)

type LoadFunc func(containerId string) (spec.Spec, error)
type IgnoreRemoveErrorFunc func(containerSpec spec.Spec, err error) bool

func SealAndLoad(containerId string, load LoadFunc) (spec.Spec, error) {
	beforeLoadedHash, err := Current(containerId)
	if err != nil {
		return spec.Spec{}, err
	}
	if err := Write(containerId, beforeLoadedHash); err != nil {
		return spec.Spec{}, err
	}

	specFile, err := load(containerId)
	if err != nil {
		return spec.Spec{}, err
	}

	afterLoadedHash, err := Current(containerId)
	if err != nil {
		return spec.Spec{}, err
	}
	sealedHash, err := Read(containerId)
	if err != nil {
		return spec.Spec{}, err
	}
	if beforeLoadedHash != sealedHash {
		return spec.Spec{}, validationError(beforeLoadedHash, sealedHash)
	}
	if sealedHash != afterLoadedHash {
		return spec.Spec{}, validationError(sealedHash, afterLoadedHash)
	}

	return specFile, nil
}

func VerifyAndLoad(containerId string, load LoadFunc) (spec.Spec, error) {
	if err := Verify(containerId); err != nil {
		return spec.Spec{}, err
	}
	return load(containerId)
}

func VerifyLoadAndConsume(containerId string, load LoadFunc, ignoreRemoveError IgnoreRemoveErrorFunc) (spec.Spec, error) {
	specFile, err := VerifyAndLoad(containerId, load)
	if err != nil {
		return spec.Spec{}, err
	}

	if err := os.Remove(utils.ConfigFileHashPath(containerId)); err != nil {
		if ignoreRemoveError == nil || !ignoreRemoveError(specFile, err) {
			return spec.Spec{}, err
		}
	}

	return specFile, nil
}

func WriteCurrent(containerId string) error {
	hash, err := Current(containerId)
	if err != nil {
		return err
	}
	return Write(containerId, hash)
}

func Verify(containerId string) error {
	expectedHash, err := Read(containerId)
	if err != nil {
		return err
	}
	currentHash, err := Current(containerId)
	if err != nil {
		return err
	}
	if expectedHash != currentHash {
		return validationError(expectedHash, currentHash)
	}
	return nil
}

func Current(containerId string) (string, error) {
	return utils.Sha256File(utils.ConfigFilePath(containerId))
}

func Read(containerId string) (string, error) {
	var specFileHash spec.SpecHash
	if err := utils.ReadJsonFile(utils.ConfigFileHashPath(containerId), &specFileHash); err != nil {
		return "", err
	}
	return specFileHash.Sha256, nil
}

func Write(containerId string, hash string) error {
	return utils.WriteJsonToFile(utils.ConfigFileHashPath(containerId), spec.SpecHash{Sha256: hash})
}

func validationError(expected string, got string) error {
	return fmt.Errorf("config.json hash validation failed: expect=%s, got=%s", expected, got)
}
