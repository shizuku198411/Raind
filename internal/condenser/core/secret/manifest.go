package secret

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"

	"raind/internal/condenser/store/sec"

	"gopkg.in/yaml.v3"
)

type Manifest struct {
	Name      string
	Namespace string
	Type      string
	Data      map[string]string
}

type manifestMeta struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type secretManifest struct {
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   manifestMeta      `yaml:"metadata"`
	Type       string            `yaml:"type"`
	Data       map[string]string `yaml:"data"`
	StringData map[string]string `yaml:"stringData"`
}

func DecodeK8sSecretManifest(body []byte) (Manifest, error) {
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
		if kind != "Secret" {
			return Manifest{}, fmt.Errorf("unsupported kind: %s", kind)
		}
		rawBytes, err := yaml.Marshal(raw)
		if err != nil {
			return Manifest{}, err
		}
		var sm secretManifest
		if err := yaml.Unmarshal(rawBytes, &sm); err != nil {
			return Manifest{}, err
		}
		return buildManifest(sm)
	}
	return Manifest{}, fmt.Errorf("secret manifest not found")
}

func buildManifest(sm secretManifest) (Manifest, error) {
	if sm.Metadata.Name == "" {
		return Manifest{}, fmt.Errorf("secret name is required")
	}
	if sm.Metadata.Namespace == "" {
		sm.Metadata.Namespace = "default"
	}
	if sm.Type == "" {
		sm.Type = sec.SecretTypeOpaque
	}
	if sm.Type != sec.SecretTypeOpaque {
		return Manifest{}, fmt.Errorf("unsupported secret type: %s", sm.Type)
	}
	data := map[string]string{}
	for k, v := range sm.Data {
		decoded, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return Manifest{}, fmt.Errorf("secret data %q is not valid base64", k)
		}
		data[k] = string(decoded)
	}
	for k, v := range sm.StringData {
		data[k] = v
	}
	return Manifest{
		Name:      sm.Metadata.Name,
		Namespace: sm.Metadata.Namespace,
		Type:      sm.Type,
		Data:      data,
	}, nil
}
