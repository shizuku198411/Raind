package namespace

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpclient "raind/internal/raind/core/client"
)

func NewServiceNamespaceCreate() *ServiceNamespaceCreate {
	return &ServiceNamespaceCreate{}
}

type ServiceNamespaceCreate struct{}

func (s *ServiceNamespaceCreate) Create(param CreateModel) (NamespaceInfoModel, error) {
	requestBody, err := json.Marshal(CreateRequestModel{Name: param.Name, Network: param.Network})
	if err != nil {
		return NamespaceInfoModel{}, err
	}
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return NamespaceInfoModel{}, err
	}
	if err := httpClient.NewRequest(http.MethodPost, "/v1/namespaces", requestBody); err != nil {
		return NamespaceInfoModel{}, err
	}
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return NamespaceInfoModel{}, fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel CreateResponseModel
	if !httpClient.IsStatusOk(resp) {
		if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
			return NamespaceInfoModel{}, fmt.Errorf("decode response: %w", err)
		}
		return NamespaceInfoModel{}, fmt.Errorf("%s", respModel.Message)
	}
	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return NamespaceInfoModel{}, fmt.Errorf("decode response: %w", err)
	}
	return respModel.Data, nil
}
