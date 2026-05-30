package container

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpclient "raind/internal/core/client"
)

func NewServiceContainerCreate() *ServiceContainerCreate {
	return &ServiceContainerCreate{}
}

type ServiceContainerCreate struct{}

func (s *ServiceContainerCreate) Create(param ServiceCreateModel) (string, error) {
	// request body
	requestBody, err := json.Marshal(
		CreateRequestModel{
			Image:   param.Image,
			Command: param.Command,
			Network: param.Network,
			Volume:  param.Volume,
			Publish: param.Publish,
			Device:  param.Device,
			Env:     param.Env,
			CapAdd:  param.CapAdd,
			CapDrop: param.CapDrop,
			Tty:     param.Tty,
			Name:    param.Name,
			PodId:   param.PodId,
		},
	)
	if err != nil {
		return "", err
	}

	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return "", fmt.Errorf("sudo required")
	}
	httpClient.NewRequest(
		http.MethodPost,
		"/v1/containers",
		requestBody,
	)
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

	return respModel.Data.Id, nil
}
