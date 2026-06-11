package network

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	httpclient "raind/internal/raind/core/client"
	"text/tabwriter"
)

func NewServiceNetworkList() *ServiceNetworkList {
	return &ServiceNetworkList{}
}

type ServiceNetworkList struct{}

func (s *ServiceNetworkList) List() error {
	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return fmt.Errorf("sudo required")
	}
	if err := httpClient.NewRequest(
		http.MethodGet,
		"/v1/networks",
		nil,
	); err != nil {
		return err
	}
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel ListResponseModel

	if !httpClient.IsStatusOk(resp) {
		decodeErr := json.NewDecoder(resp.Body).Decode(&respModel)
		if decodeErr != nil {
			return fmt.Errorf("decode response: %w", decodeErr)
		}
		return fmt.Errorf("unexpected status: %s: %s", resp.Status, respModel.Message)
	}

	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	s.printNetworkList(respModel.Data)

	return nil
}

func (s *ServiceNetworkList) printNetworkList(networks []NetworkInfoModel) {
	w := tabwriter.NewWriter(
		os.Stdout,
		0,
		0,
		2,
		' ',
		tabwriter.DiscardEmptyColumns,
	)

	fmt.Fprintln(w, "NETWORK\tADDRESS\tCONTAINERS")
	for _, n := range networks {
		fmt.Fprintf(w, "%s\t%s\t%d\n", n.Interface, n.Address, n.NumContainers)
	}

	w.Flush()
}
