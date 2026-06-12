package pod

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpclient "raind/internal/raind/core/client"
)

func NewServicePodRemove() *ServicePodRemove {
	return &ServicePodRemove{}
}

type ServicePodRemove struct{}

func (s *ServicePodRemove) Remove(param ServicePodRemoveModel) error {
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	if err := httpClient.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("/v1/pods/%s", param.Id),
		nil,
	); err != nil {
		return err
	}
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

	podId := param.Id
	if respModel.Data.PodId != "" {
		podId = respModel.Data.PodId
	} else if respModel.Data.Id != "" {
		podId = respModel.Data.Id
	}

	fmt.Printf("pod: %s removed\n", podId)
	return nil
}
