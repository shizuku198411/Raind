package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpclient "raind/internal/raind/core/client"
)

func NewServiceNetworkCreate() *ServiceNetworkCreate {
	return &ServiceNetworkCreate{}
}

type ServiceNetworkCreate struct{}

func (s *ServiceNetworkCreate) Create(param ServiceNetworkCreateModel) (string, error) {
	// request body
	requestBody, err := json.Marshal(
		CreateRequestModel{
			Bridge: param.Bridge,
		},
	)
	if err != nil {
		return "", err
	}

	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return "", err
	}

	if err := httpClient.NewRequest(
		http.MethodPost,
		"/v1/networks",
		requestBody,
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

	return respModel.Data.Bridge, nil
}
