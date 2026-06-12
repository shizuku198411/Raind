package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	httpclient "raind/internal/raind/core/client"
)

func NewServiceServiceCreate() *ServiceServiceCreate {
	return &ServiceServiceCreate{}
}

type ServiceServiceCreate struct{}

func (s *ServiceServiceCreate) Create(param ServiceServiceCreateModel) (string, error) {
	if param.FilePath == "" {
		return "", fmt.Errorf("yaml file path is required")
	}

	bodyBytes, err := os.ReadFile(param.FilePath)
	if err != nil {
		return "", fmt.Errorf("read yaml file: %w", err)
	}

	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return "", err
	}

	endpoint := httpClient.BaseUrl + "/v1/services"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := httpClient.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel CreateResponseModel

	if !httpClient.IsStatusOk(resp) {
		if decodeErr := json.NewDecoder(resp.Body).Decode(&respModel); decodeErr != nil {
			return "", fmt.Errorf("decode response: %w", decodeErr)
		}
		return "", fmt.Errorf("%s", respModel.Message)
	}

	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return respModel.Data.ServiceId, nil
}
