package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func SafeJoin(root string, elems ...string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target := filepath.Join(append([]string{absRoot}, elems...)...)
	if err := EnsurePathUnderRoot(absRoot, target); err != nil {
		return "", err
	}
	return filepath.Abs(target)
}

func EnsurePathUnderRoot(root, path string) error {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absRoot, target)
	if err != nil {
		return err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("path escapes root %q: %s", root, path)
	}
	return nil
}

func RemoveAllUnderRoot(fs FilesystemHandler, root, path string) error {
	if err := EnsurePathUnderRoot(root, path); err != nil {
		return err
	}
	return fs.RemoveAll(path)
}
