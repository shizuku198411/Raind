package promote

import (
	"fmt"
	"os"
)

func WriteOutput(path string, data []byte, force bool) error {
	if path == "" {
		path = "bottle.yaml"
	}
	if _, err := os.Stat(path); err == nil && !force {
		return fmt.Errorf("output file already exists: %s (use --force to overwrite)", path)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
