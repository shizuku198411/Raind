package configmap

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

func (s *ServiceRemove) Remove(idOrName, namespace string) (ConfigMapInfo, error) {
	if idOrName == "" {
		return ConfigMapInfo{}, fmt.Errorf("configmap id or name is required")
	}
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return ConfigMapInfo{}, err
	}
	path := "/v1/configmaps/" + url.PathEscape(idOrName)
	if namespace != "" {
		path += "?namespace=" + url.QueryEscape(namespace)
	}
	if err := httpClient.NewRequest(http.MethodDelete, path, nil); err != nil {
		return ConfigMapInfo{}, err
	}
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return ConfigMapInfo{}, fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel RemoveResponseModel
	if !httpClient.IsStatusOk(resp) {
		if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
			return ConfigMapInfo{}, fmt.Errorf("decode response: %w", err)
		}
		return ConfigMapInfo{}, fmt.Errorf("%s", respModel.Message)
	}
	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return ConfigMapInfo{}, fmt.Errorf("decode response: %w", err)
	}
	return respModel.Data, nil
}
