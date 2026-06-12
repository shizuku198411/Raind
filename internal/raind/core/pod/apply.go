package pod

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	httpclient "raind/internal/raind/core/client"
)

func NewServicePodApply() *ServicePodApply {
	return &ServicePodApply{}
}

type ServicePodApply struct{}

func (s *ServicePodApply) Apply(param ServicePodApplyModel) (ApplyResponseDataModel, error) {
	if param.FilePath == "" {
		return ApplyResponseDataModel{}, fmt.Errorf("yaml file path is required")
	}

	yamlBytes, err := os.ReadFile(param.FilePath)
	if err != nil {
		return ApplyResponseDataModel{}, fmt.Errorf("read yaml file: %w", err)
	}

	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return ApplyResponseDataModel{}, err
	}

	endpoint := httpClient.BaseUrl + "/v1/resource/apply"
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(yamlBytes))
	if err != nil {
		return ApplyResponseDataModel{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/yaml")

	resp, err := httpClient.Client.Do(req)
	if err != nil {
		return ApplyResponseDataModel{}, fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel ApplyResponseModel

	if !httpClient.IsStatusOk(resp) {
		if decodeErr := json.NewDecoder(resp.Body).Decode(&respModel); decodeErr != nil {
			return ApplyResponseDataModel{}, fmt.Errorf("decode response: %w", decodeErr)
		}
		return ApplyResponseDataModel{}, fmt.Errorf("%s", respModel.Message)
	}

	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return ApplyResponseDataModel{}, fmt.Errorf("decode response: %w", err)
	}

	return respModel.Data, nil
}
