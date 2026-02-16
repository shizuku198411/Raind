package pod

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpclient "raind/internal/core/client"
)

func NewServicePodCreate() *ServicePodCreate {
	return &ServicePodCreate{}
}

type ServicePodCreate struct{}

func (s *ServicePodCreate) Create(param ServicePodCreateModel) (string, error) {
	requestBody, err := json.Marshal(
		CreateRequestModel{
			Name:        param.Name,
			Namespace:   param.Namespace,
			UID:         param.UID,
			Labels:      param.Labels,
			Annotations: param.Annotations,
		},
	)
	if err != nil {
		return "", err
	}

	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return "", fmt.Errorf("sudo required")
	}
	if err := httpClient.NewRequest(
		http.MethodPost,
		"/v1/pods",
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

	if respModel.Data.PodId != "" {
		return respModel.Data.PodId, nil
	}
	return respModel.Data.Id, nil
}
