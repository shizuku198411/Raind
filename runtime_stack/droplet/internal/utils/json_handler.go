package utils

import (
	"encoding/json"
	"os"
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
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "    ")
	return encoder.Encode(v)
}

func ReadJsonFile(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
