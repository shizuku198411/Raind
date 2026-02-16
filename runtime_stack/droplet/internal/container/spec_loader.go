package container

import (
	"droplet/internal/spec"
	"droplet/internal/utils"
)

// specLoader loads an OCI runtime specification for a container.
//
// Implementations are responsible for retrieving and parsing the
// container's config.json (OCI runtime spec) based on a container ID.
type specLoader interface {
	loadFile(containerId string) (spec.Spec, error)
}

// newFileSpecLoader returns a fileSpecLoader, which loads container
// specifications from the local filesystem.
//
// This is the default implementation used by the runtime.
func newFileSpecLoader() *fileSpecLoader {
	return &fileSpecLoader{}
}

// fileSpecLoader loads an OCI spec from a config.json file on disk.
//
// The loader resolves the config file path for a given container ID
// and delegates spec parsing to spec.LoadConfigFile.
type fileSpecLoader struct{}

// loadFile loads and parses the OCI runtime specification for the
// specified container ID.
//
// The configuration is read from the container's config.json file.
// An error is returned if the file cannot be read or parsed.
func (f *fileSpecLoader) loadFile(containerId string) (spec.Spec, error) {
	path := utils.ConfigFilePath(containerId)
	return spec.LoadConfigFile(path)
}
