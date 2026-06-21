package promote

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const BottleReviewFileName = "REVIEW_BOTTLE.md"

func WriteOutput(path string, data []byte, force bool) error {
	if path == "" {
		path = "bottle.yaml"
	}
	if err := ensureWritablePath(path, force); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func WriteBottlePromotionOutputs(path string, bottleData, reviewData []byte, force bool) error {
	if path == "" {
		path = "bottle.yaml"
	}
	reviewPath := BottleReviewPathForOutput(path)
	if err := ensureWritablePath(path, force); err != nil {
		return err
	}
	if err := ensureWritablePath(reviewPath, force); err != nil {
		return err
	}
	if err := os.WriteFile(path, bottleData, 0644); err != nil {
		return err
	}
	return os.WriteFile(reviewPath, reviewData, 0644)
}

func BottleReviewPathForOutput(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		path = "bottle.yaml"
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return BottleReviewFileName
	}
	return filepath.Join(dir, BottleReviewFileName)
}

func ensureWritablePath(path string, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("output file already exists: %s (use --force to overwrite)", path)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
