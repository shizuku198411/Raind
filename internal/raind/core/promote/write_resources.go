package promote

import (
	"fmt"
	"os"
	"path/filepath"
)

func WriteResourceFiles(outputDir string, files []ResourceFile, force bool) error {
	if outputDir == "" {
		outputDir = "manifests"
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	for _, file := range files {
		if file.Name == "" {
			continue
		}
		path := filepath.Join(outputDir, file.Name)
		if _, err := os.Stat(path); err == nil && !force {
			return fmt.Errorf("output file already exists: %s (use --force to overwrite)", path)
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
		if err := os.WriteFile(path, file.Data, 0644); err != nil {
			return err
		}
	}
	return nil
}
