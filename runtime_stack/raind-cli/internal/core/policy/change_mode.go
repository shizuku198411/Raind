package policy

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpclient "raind/internal/core/client"
)

func NewServicePolicyChangeMode() *ServicePolicyChangeMode {
	return &ServicePolicyChangeMode{}
}

type ServicePolicyChangeMode struct{}

func (s *ServicePolicyChangeMode) ChangeMode(param ServiceChangeModeModel) error {
	// request body
	requestBody, err := json.Marshal(
		ChangeModeRequestModel{
			Mode: param.Mode,
		},
	)
	if err != nil {
		return err
	}

	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return fmt.Errorf("sudo required")
	}
	httpClient.NewRequest(
		http.MethodPost,
		"/v1/policies/ns/mode",
		requestBody,
	)
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel ChangeModeResponseModel

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

	fmt.Printf("policy north-south mode changed: %s\n", respModel.Data.Mode)

	return nil
}
