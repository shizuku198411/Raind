package container

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpclient "raind/internal/core/client"
)

func NewServiceContainerRemove() *ServiceContainerRemove {
	return &ServiceContainerRemove{}
}

type ServiceContainerRemove struct{}

func (s *ServiceContainerRemove) Remove(param ServiceRemoveModel) error {
	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return fmt.Errorf("sudo required")
	}
	httpClient.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("/v1/containers/%s/actions/delete", param.Id),
		nil,
	)
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel RemoveResponseModel

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

	fmt.Printf("container: %s removed\n", param.Id)

	return nil
}
