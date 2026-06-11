package replicaset

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpclient "raind/internal/raind/core/client"
)

func NewServiceReplicaSetRemove() *ServiceReplicaSetRemove {
	return &ServiceReplicaSetRemove{}
}

type ServiceReplicaSetRemove struct{}

func (s *ServiceReplicaSetRemove) Remove(param ServiceReplicaSetRemoveModel) (string, error) {
	if param.Id == "" {
		return "", fmt.Errorf("replicaset id is required")
	}

	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return "", fmt.Errorf("sudo required")
	}
	if err := httpClient.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("/v1/replicasets/%s", param.Id),
		nil,
	); err != nil {
		return "", err
	}
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return "", fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel RemoveResponseModel

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

	if respModel.Data.ReplicaSetId != "" {
		return respModel.Data.ReplicaSetId, nil
	}
	return param.Id, nil
}
