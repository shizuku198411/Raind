package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func JsonToString(v any) (string, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func StringToJson(s string, v any) error {
	return json.Unmarshal([]byte(s), v)
}

func WriteJsonToFile(path string, v any) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := f.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "    ")
	if err := encoder.Encode(v); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Chmod(0o644); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	committed = true
	return nil
}

func ReadJsonFile(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
