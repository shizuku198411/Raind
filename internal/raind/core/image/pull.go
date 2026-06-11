package image

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpclient "raind/internal/raind/core/client"
)

func NewServiceImagePull() *ServiceImagePull {
	return &ServiceImagePull{}
}

type ServiceImagePull struct{}

func (s *ServiceImagePull) Pull(param ServiceImagePullModel) error {
	// request body
	requestBody, err := json.Marshal(
		ImagePullRequestModel{
			Image: param.Image,
			Os:    param.Os,
			Arch:  param.Arch,
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
		http.MethodPost,
		"/v1/images?stream=1",
		requestBody,
	)
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel ImagePullResponseModel

	if !httpClient.IsStatusOk(resp) {
		decodeErr := json.NewDecoder(resp.Body).Decode(&respModel)
		if decodeErr != nil {
			return fmt.Errorf("decode response: %w", decodeErr)
		}
		return fmt.Errorf("unexpected status: %s: %s", resp.Status, respModel.Message)
	}

	if resp.Header.Get("Content-Type") == "application/x-ndjson" {
		if _, err := httpclient.ReadStreamEvents(resp.Body); err != nil {
			return err
		}
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}
