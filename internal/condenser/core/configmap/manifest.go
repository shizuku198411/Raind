package configmap

import (
	"bytes"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

type Manifest struct {
	Name      string
	Namespace string
	Data      map[string]string
	Warnings  []Warning
}

type Warning struct {
	Field   string
	Message string
}

type manifestMeta struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type configMapManifest struct {
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   manifestMeta      `yaml:"metadata"`
	Data       map[string]string `yaml:"data"`
	BinaryData map[string]string `yaml:"binaryData"`
	Immutable  *bool             `yaml:"immutable"`
}

func DecodeK8sConfigMapManifest(body []byte) (Manifest, error) {
	dec := yaml.NewDecoder(bytes.NewReader(body))
	for {
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			return Manifest{}, err
		}
		if len(raw) == 0 {
			continue
		}
		kind, _ := raw["kind"].(string)
		if kind == "" {
			return Manifest{}, fmt.Errorf("kind is required")
		}
		if kind != "ConfigMap" {
			return Manifest{}, fmt.Errorf("unsupported kind: %s", kind)
		}
		rawBytes, err := yaml.Marshal(raw)
		if err != nil {
			return Manifest{}, err
		}
		var cm configMapManifest
		if err := yaml.Unmarshal(rawBytes, &cm); err != nil {
			return Manifest{}, err
		}
		if cm.Metadata.Name == "" {
			return Manifest{}, fmt.Errorf("configmap name is required")
		}
		if cm.Metadata.Namespace == "" {
			cm.Metadata.Namespace = "default"
		}
		data := map[string]string{}
		for k, v := range cm.Data {
			data[k] = v
		}
		var warnings []Warning
		if len(cm.BinaryData) > 0 {
			warnings = append(warnings, Warning{Field: "binaryData", Message: "binaryData is ignored in the current ConfigMap implementation"})
		}
		if cm.Immutable != nil {
			warnings = append(warnings, Warning{Field: "immutable", Message: "immutable update semantics are not enforced"})
		}
		return Manifest{
			Name:      cm.Metadata.Name,
			Namespace: cm.Metadata.Namespace,
			Data:      data,
			Warnings:  warnings,
		}, nil
	}
	return Manifest{}, fmt.Errorf("configmap manifest not found")
}
