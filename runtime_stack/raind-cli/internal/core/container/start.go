package container

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpclient "raind/internal/core/client"
)

func NewServiceContainerStart() *ServiceContainerStart {
	return &ServiceContainerStart{}
}

type ServiceContainerStart struct{}

func (s *ServiceContainerStart) Start(param ServiceStartModel) error {
	// request body
	requestBody, err := json.Marshal(
		StartRequestModel{
			Tty: param.Tty,
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
		fmt.Sprintf("/v1/containers/%s/actions/start", param.Id),
		requestBody,
	)
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel StartResponseModel

	if !httpClient.IsStatusOk(resp) {
		decodeErr := json.NewDecoder(resp.Body).Decode(&respModel)
		if decodeErr != nil {
			return fmt.Errorf("decode response: %w", decodeErr)
		}
		return fmt.Errorf("%s", respModel.Message)
	}

	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	fmt.Printf("container: %s started\n", param.Id)

	return nil
}
