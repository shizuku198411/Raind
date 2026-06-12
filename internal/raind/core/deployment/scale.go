package deployment

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpclient "raind/internal/raind/core/client"
)

func NewServiceDeploymentScale() *ServiceDeploymentScale {
	return &ServiceDeploymentScale{}
}

type ServiceDeploymentScale struct{}

func (s *ServiceDeploymentScale) Scale(param ServiceDeploymentScaleModel) (ScaleResponseDataModel, error) {
	if param.Id == "" {
		return ScaleResponseDataModel{}, fmt.Errorf("deployment id is required")
	}

	requestBody, err := json.Marshal(ScaleRequestModel{Replicas: param.Replicas})
	if err != nil {
		return ScaleResponseDataModel{}, err
	}

	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return ScaleResponseDataModel{}, err
	}
	if err := httpClient.NewRequest(http.MethodPost, fmt.Sprintf("/v1/deployments/%s/actions/scale", param.Id), requestBody); err != nil {
		return ScaleResponseDataModel{}, err
	}
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return ScaleResponseDataModel{}, fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel ScaleResponseModel
	if !httpClient.IsStatusOk(resp) {
		if decodeErr := json.NewDecoder(resp.Body).Decode(&respModel); decodeErr != nil {
			return ScaleResponseDataModel{}, fmt.Errorf("decode response: %w", decodeErr)
		}
		return ScaleResponseDataModel{}, fmt.Errorf("%s", respModel.Message)
	}
	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return ScaleResponseDataModel{}, fmt.Errorf("decode response: %w", err)
	}

	return respModel.Data, nil
}
