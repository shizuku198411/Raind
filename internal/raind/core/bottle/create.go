package bottle

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpclient "raind/internal/raind/core/client"
)

func NewServiceBottleCreate() *ServiceBottleCreate {
	return &ServiceBottleCreate{}
}

type ServiceBottleCreate struct{}

func (s *ServiceBottleCreate) Create(param ServiceBottleCreateModel) (CreateResponseDataModel, error) {
	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return CreateResponseDataModel{}, fmt.Errorf("sudo required")
	}

	if err := httpClient.NewRequest(
		http.MethodPost,
		"/v1/bottle",
		param.Yaml,
	); err != nil {
		return CreateResponseDataModel{}, err
	}
	httpClient.Request.Header.Set("Content-Type", "text/plain")

	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return CreateResponseDataModel{}, fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel CreateResponseModel

	if !httpClient.IsStatusOk(resp) {
		decodeErr := json.NewDecoder(resp.Body).Decode(&respModel)
		if decodeErr != nil {
			return CreateResponseDataModel{}, fmt.Errorf("decode response: %w", decodeErr)
		}
		return CreateResponseDataModel{}, fmt.Errorf("%s", respModel.Message)
	}

	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return CreateResponseDataModel{}, fmt.Errorf("decode response: %w", err)
	}

	return respModel.Data, nil
}
