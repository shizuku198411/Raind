package resource

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

type Header struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
}

func decodeHeader(rawBytes []byte) (Header, error) {
	var header Header
	if err := yaml.Unmarshal(rawBytes, &header); err != nil {
		return Header{}, err
	}
	if header.Kind == "" {
		return Header{}, fmt.Errorf("kind is required")
	}
	return header, nil
}

type namespaceManifest struct {
	APIVersion string        `yaml:"apiVersion"`
	Kind       string        `yaml:"kind"`
	Metadata   namespaceMeta `yaml:"metadata"`
}

type namespaceMeta struct {
	Name        string            `yaml:"name"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

func decodeNamespaceManifest(rawBytes []byte) (namespaceManifest, error) {
	var manifest namespaceManifest
	if err := yaml.Unmarshal(rawBytes, &manifest); err != nil {
		return namespaceManifest{}, err
	}
	if manifest.Metadata.Name == "" {
		return namespaceManifest{}, fmt.Errorf("namespace name is required")
	}
	return manifest, nil
}

func invalidYAMLErrorMessage(err error) string {
	if err == nil {
		return "invalid yaml"
	}
	return "invalid yaml: " + err.Error()
}
