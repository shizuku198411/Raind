package securityprofile

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func readManifestFile(path string) (CustomProfileManifest, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return CustomProfileManifest{}, err
	}

	var manifest CustomProfileManifest
	if err := yaml.Unmarshal(body, &manifest); err != nil {
		return CustomProfileManifest{}, fmt.Errorf("parse security profile manifest %q: %w", path, err)
	}
	return manifest, nil
}

func writeManifestFile(path string, manifest CustomProfileManifest) error {
	body, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode security profile manifest: %w", err)
	}
	if err := os.WriteFile(path, body, 0644); err != nil {
		return fmt.Errorf("write security profile manifest %q: %w", path, err)
	}
	return nil
}
