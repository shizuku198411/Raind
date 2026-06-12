package deployment

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpclient "raind/internal/raind/core/client"
)

func NewServiceDeploymentRemove() *ServiceDeploymentRemove {
	return &ServiceDeploymentRemove{}
}

type ServiceDeploymentRemove struct{}

func (s *ServiceDeploymentRemove) Remove(param ServiceDeploymentRemoveModel) (string, error) {
	if param.Id == "" {
		return "", fmt.Errorf("deployment id is required")
	}

	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return "", err
	}
	if err := httpClient.NewRequest(http.MethodDelete, fmt.Sprintf("/v1/deployments/%s", param.Id), nil); err != nil {
		return "", err
	}
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return "", fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel RemoveResponseModel
	if !httpClient.IsStatusOk(resp) {
		if decodeErr := json.NewDecoder(resp.Body).Decode(&respModel); decodeErr != nil {
			return "", fmt.Errorf("decode response: %w", decodeErr)
		}
		return "", fmt.Errorf("%s", respModel.Message)
	}
	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if respModel.Data.DeploymentId != "" {
		return respModel.Data.DeploymentId, nil
	}
	return param.Id, nil
}
