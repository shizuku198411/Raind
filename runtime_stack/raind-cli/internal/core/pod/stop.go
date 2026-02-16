package pod

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpclient "raind/internal/core/client"
)

func NewServicePodStop() *ServicePodStop {
	return &ServicePodStop{}
}

type ServicePodStop struct{}

func (s *ServicePodStop) Stop(param ServicePodStopModel) error {
	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return fmt.Errorf("sudo required")
	}
	if err := httpClient.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/v1/pods/%s/actions/stop", param.Id),
		nil,
	); err != nil {
		return err
	}
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel StopResponseModel

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

	fmt.Printf("pod: %s stopped\n", podId)
	return nil
}
