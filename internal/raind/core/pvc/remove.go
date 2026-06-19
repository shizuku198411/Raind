package pvc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	httpclient "raind/internal/raind/core/client"
)

func NewServiceRemove() *ServiceRemove {
	return &ServiceRemove{}
}

type ServiceRemove struct{}

func (s *ServiceRemove) Remove(idOrName, namespace string) error {
	if idOrName == "" {
		return fmt.Errorf("persistentvolumeclaim id or name is required")
	}
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	path := "/v1/persistentvolumeclaims/" + url.PathEscape(idOrName)
	if namespace != "" {
		path += "?namespace=" + url.QueryEscape(namespace)
	}
	if err := httpClient.NewRequest(http.MethodDelete, path, nil); err != nil {
		return err
	}
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel RemoveResponseModel
	if !httpClient.IsStatusOk(resp) {
		if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		return fmt.Errorf("unexpected status: %s: %s", resp.Status, respModel.Message)
	}
	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	fmt.Printf("persistentvolumeclaim: %s removed\n", respModel.Data.PVCId)
	return nil
}
