package promote

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	DefaultBottlePromotionOutput  = "raind_promote/bottle/bottle.yaml"
	DefaultComposePromotionOutput = "raind_promote/compose/compose.yaml"
	BottleReviewFileName          = "REVIEW_BOTTLE.md"
)

func WriteOutput(path string, data []byte, force bool) error {
	if path == "" {
		path = DefaultBottlePromotionOutput
	}
	if err := ensureParentDir(path); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func WriteBottlePromotionOutputs(path string, bottleData, reviewData []byte, force bool) error {
	if path == "" {
		path = DefaultBottlePromotionOutput
	}
	reviewPath := BottleReviewPathForOutput(path)
	if err := ensureParentDir(path); err != nil {
		return err
	}
	if err := ensureParentDir(reviewPath); err != nil {
		return err
	}
	if err := os.WriteFile(path, bottleData, 0644); err != nil {
		return err
	}
	return os.WriteFile(reviewPath, reviewData, 0644)
}

func WriteComposePromotionOutput(data []byte, force bool) error {
	if err := ensureParentDir(DefaultComposePromotionOutput); err != nil {
		return err
	}
	return os.WriteFile(DefaultComposePromotionOutput, data, 0644)
}

func BottleReviewPathForOutput(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		path = DefaultBottlePromotionOutput
	}
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return BottleReviewFileName
	}
	return filepath.Join(dir, BottleReviewFileName)
}

func ensureParentDir(path string) error {
	dir := filepath.Dir(strings.TrimSpace(path))
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}
