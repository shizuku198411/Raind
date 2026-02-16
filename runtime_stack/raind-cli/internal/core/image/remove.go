package image

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpclient "raind/internal/core/client"
)

func NewServiceImageRemove() *ServiceImageRemove {
	return &ServiceImageRemove{}
}

type ServiceImageRemove struct{}

func (s *ServiceImageRemove) Remove(param ServiceImageRemoveModel) error {
	// request body
	requestBody, err := json.Marshal(
		ImageRemoveRequestModel{
			Image: param.Image,
		},
	)
	if err != nil {
		return err
	}

	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return fmt.Errorf("sudo required")
	}
	httpClient.NewRequest(
		http.MethodDelete,
		"/v1/images",
		requestBody,
	)
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel ImageRemoveResponseModel

	if !httpClient.IsStatusOk(resp) {
		decodeErr := json.NewDecoder(resp.Body).Decode(&respModel)
		if decodeErr != nil {
			return fmt.Errorf("decode response: %w", decodeErr)
		}
		return fmt.Errorf("unexpected status: %s: %s", resp.Status, respModel.Message)
	}

	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}
