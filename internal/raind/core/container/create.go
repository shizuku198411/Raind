package container

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpclient "raind/internal/raind/core/client"
)

func NewServiceContainerCreate() *ServiceContainerCreate {
	return &ServiceContainerCreate{}
}

type ServiceContainerCreate struct{}

func (s *ServiceContainerCreate) Create(param ServiceCreateModel) (string, error) {
	// request body
	requestBody, err := json.Marshal(
		CreateRequestModel{
			Image:           param.Image,
			Command:         param.Command,
			Network:         param.Network,
			Volume:          param.Volume,
			Publish:         param.Publish,
			Device:          param.Device,
			Env:             param.Env,
			CapAdd:          param.CapAdd,
			CapDrop:         param.CapDrop,
			Tty:             param.Tty,
			Name:            param.Name,
			Rootless:        param.Rootless,
			RootlessMode:    param.RootlessMode,
			RootlessRootUID: param.RootlessRootUID,
			RootlessRootGID: param.RootlessRootGID,
			PodId:           param.PodId,
		},
	)
	if err != nil {
		return "", err
	}

	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return "", err
	}
	httpClient.NewRequest(
		http.MethodPost,
		"/v1/containers?stream=1",
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

	if resp.Header.Get("Content-Type") == "application/x-ndjson" {
		event, err := httpclient.ReadStreamEvents(resp.Body)
		if err != nil {
			return "", err
		}
		if len(event.Data) > 0 {
			if err := json.Unmarshal(event.Data, &respModel.Data); err != nil {
				var createResp CreateResponseDataModel
				if decodeErr := json.Unmarshal(event.Data, &createResp); decodeErr != nil {
					return "", fmt.Errorf("decode stream response: %w", decodeErr)
				}
				respModel.Data = createResp
			}
		}
		if respModel.Data.Id == "" {
			respModel.Data.Id = event.ID
		}
		return respModel.Data.Id, nil
	}

	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return respModel.Data.Id, nil
}
