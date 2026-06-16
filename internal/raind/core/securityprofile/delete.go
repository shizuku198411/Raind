package securityprofile

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	httpclient "raind/internal/raind/core/client"
)

func NewServiceDelete() *ServiceDelete {
	return &ServiceDelete{}
}

type ServiceDelete struct{}

func (s *ServiceDelete) Delete(name string) error {
	if name == "" {
		return fmt.Errorf("missing security profile name")
	}

	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	if err := httpClient.NewRequest(http.MethodDelete, "/v1/security/profiles/"+url.PathEscape(name), nil); err != nil {
		return err
	}

	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel DeleteResponseModel
	if !httpClient.IsStatusOk(resp) {
		if decodeErr := json.NewDecoder(resp.Body).Decode(&respModel); decodeErr != nil {
			return fmt.Errorf("decode response: %w", decodeErr)
		}
		return fmt.Errorf("unexpected status: %s: %s", resp.Status, respModel.Message)
	}

	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	fmt.Printf("security profile: %s deleted\n", respModel.Data.Name)
	return nil
}
