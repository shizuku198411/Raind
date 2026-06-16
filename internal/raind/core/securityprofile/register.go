package securityprofile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	httpclient "raind/internal/raind/core/client"

	"gopkg.in/yaml.v3"
)

func NewServiceRegister() *ServiceRegister {
	return &ServiceRegister{}
}

type ServiceRegister struct{}

func (s *ServiceRegister) Register(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("security profile yaml file path is required")
	}

	bodyBytes, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read security profile yaml file: %w", err)
	}

	var manifest CustomProfileManifest
	if err := yaml.Unmarshal(bodyBytes, &manifest); err != nil {
		return fmt.Errorf("parse security profile yaml file: %w", err)
	}

	requestBody, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode security profile request: %w", err)
	}

	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}

	endpoint := httpClient.BaseUrl + "/v1/security/profiles"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(requestBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Client.Do(req)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel RegisterResponseModel
	if !httpClient.IsStatusOk(resp) {
		if decodeErr := json.NewDecoder(resp.Body).Decode(&respModel); decodeErr != nil {
			return fmt.Errorf("decode response: %w", decodeErr)
		}
		return fmt.Errorf("unexpected status: %s: %s", resp.Status, respModel.Message)
	}

	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	fmt.Printf("security profile: %s registered\n", respModel.Data.Profile.Name)
	return nil
}
