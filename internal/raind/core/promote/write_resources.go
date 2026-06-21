package promote

import (
	"os"
	"path/filepath"
)

const DefaultResourcePromotionOutput = "raind_promote/resources"

func WriteResourceFiles(outputDir string, files []ResourceFile, force bool) error {
	if outputDir == "" {
		outputDir = DefaultResourcePromotionOutput
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	for _, file := range files {
		if file.Name == "" {
			continue
		}
		path := filepath.Join(outputDir, file.Name)
		if err := os.WriteFile(path, file.Data, 0644); err != nil {
			return err
		}
	}
	return nil
}
