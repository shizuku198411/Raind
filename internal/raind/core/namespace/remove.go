package namespace

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpclient "raind/internal/raind/core/client"
)

func NewServiceNamespaceRemove() *ServiceNamespaceRemove {
	return &ServiceNamespaceRemove{}
}

type ServiceNamespaceRemove struct{}

func (s *ServiceNamespaceRemove) Remove(name string) (string, error) {
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return "", err
	}
	if err := httpClient.NewRequest(http.MethodDelete, "/v1/namespaces/"+name+"/actions/delete", nil); err != nil {
		return "", err
	}
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return "", fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel RemoveResponseModel
	if !httpClient.IsStatusOk(resp) {
		if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
			return "", fmt.Errorf("decode response: %w", err)
		}
		return "", fmt.Errorf("unexpected status: %s: %s", resp.Status, respModel.Message)
	}
	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	return name, nil
}
