package policy

import (
	"encoding/json"
	"fmt"
	"net/http"
	httpclient "raind/internal/raind/core/client"
)

func NewServicePolicyCreate() *ServicePolicyCreate {
	return &ServicePolicyCreate{}
}

type ServicePolicyCreate struct{}

func (s *ServicePolicyCreate) Create(param ServiceCreateModel) error {
	_, err := s.CreateWithID(param)
	return err
}

func (s *ServicePolicyCreate) CreateWithID(param ServiceCreateModel) (string, error) {
	var chainName string
	switch param.Chain {
	case "ew":
		chainName = "RAIND-EW"
	case "ns-obs":
		chainName = "RAIND-NS-OBS"
	case "ns-enf":
		chainName = "RAIND-NS-ENF"
	}

	// request body
	requestBody, err := json.Marshal(
		CreateRequestModel{
			Chain:       chainName,
			Source:      param.Source,
			Destination: param.Destination,
			Protocol:    param.Protocol,
			DestPort:    param.DestPort,
			Comment:     param.Comment,
		},
	)
	if err != nil {
		return "", err
	}

	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return "", err
	}
	httpClient.NewRequest(
		http.MethodPost,
		"/v1/policies",
		requestBody,
	)
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return "", fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel CreateResponseModel

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

	fmt.Printf("policy: %s created\n", respModel.Data.Id)

	return respModel.Data.Id, nil
}
