package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpclient "raind/internal/core/client"
)

func NewServiceNetworkRemove() *ServiceNetworkRemove {
	return &ServiceNetworkRemove{}
}

type ServiceNetworkRemove struct{}

func (s *ServiceNetworkRemove) Remove(param ServiceNetworkRemoveModel) (string, error) {
	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return "", fmt.Errorf("sudo required")
	}

	if err := httpClient.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("/v1/networks/%s/actions/delete", param.Bridge),
		nil,
	); err != nil {
		return "", err
	}
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return "", fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel CreateResponseModel

	if !httpClient.IsStatusOk(resp) {
		decodeErr := json.NewDecoder(resp.Body).Decode(&respModel)
		if decodeErr != nil {
			return "", fmt.Errorf("decode response: %w", decodeErr)
		}
		return "", fmt.Errorf("%s", respModel.Message)
	}

	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return respModel.Message, nil
}
