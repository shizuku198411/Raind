package securityprofile

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	httpclient "raind/internal/raind/core/client"

	"gopkg.in/yaml.v3"
)

func NewServiceShow() *ServiceShow {
	return &ServiceShow{}
}

type ServiceShow struct{}

func (s *ServiceShow) Show(name string) error {
	if name == "" {
		return fmt.Errorf("missing security profile name")
	}
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	if err := httpClient.NewRequest(http.MethodGet, "/v1/security/profiles/"+url.PathEscape(name), nil); err != nil {
		return err
	}
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel ShowResponseModel
	if !httpClient.IsStatusOk(resp) {
		if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		return fmt.Errorf("unexpected status: %s: %s", resp.Status, respModel.Message)
	}
	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	out, err := yaml.Marshal(respModel.Data.Profile)
	if err != nil {
		return fmt.Errorf("encode yaml: %w", err)
	}
	fmt.Print(string(out))
	return nil
}
