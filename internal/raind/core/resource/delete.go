package resource

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	httpclient "raind/internal/raind/core/client"
)

func NewServiceResourceDelete() *ServiceResourceDelete {
	return &ServiceResourceDelete{}
}

type ServiceResourceDelete struct{}

func (s *ServiceResourceDelete) Delete(param ServiceResourceDeleteModel) (DeleteResponseModel, error) {
	if param.FilePath == "" {
		return DeleteResponseModel{}, fmt.Errorf("yaml file path is required")
	}

	bodyBytes, err := os.ReadFile(param.FilePath)
	if err != nil {
		return DeleteResponseModel{}, fmt.Errorf("read yaml file: %w", err)
	}

	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return DeleteResponseModel{}, fmt.Errorf("sudo required")
	}

	endpoint := httpClient.BaseUrl + "/v1/resource/delete"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return DeleteResponseModel{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := httpClient.Client.Do(req)
	if err != nil {
		return DeleteResponseModel{}, fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel DeleteResponseModel
	if !httpClient.IsStatusOk(resp) {
		if decodeErr := json.NewDecoder(resp.Body).Decode(&respModel); decodeErr != nil {
			return DeleteResponseModel{}, fmt.Errorf("decode response: %w", decodeErr)
		}
		return DeleteResponseModel{}, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return DeleteResponseModel{}, fmt.Errorf("decode response: %w", err)
	}

	return respModel, nil
}
