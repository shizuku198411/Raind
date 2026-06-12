package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	httpclient "raind/internal/raind/core/client"
	"sort"
	"strings"
	"text/tabwriter"
)

func NewServiceServiceDetail() *ServiceServiceDetail {
	return &ServiceServiceDetail{}
}

type ServiceServiceDetail struct{}

func (s *ServiceServiceDetail) Detail(id string) error {
	if id == "" {
		return fmt.Errorf("service id is required")
	}

	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	if err := httpClient.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/v1/services/%s", id),
		nil,
	); err != nil {
		return err
	}
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel DetailResponseModel

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

	s.printServiceDetail(respModel.Data)
	return nil
}

func (s *ServiceServiceDetail) printServiceDetail(svc ServiceDetailModel) {
	w := tabwriter.NewWriter(
		os.Stdout,
		0,
		0,
		2,
		' ',
		tabwriter.DiscardEmptyColumns,
	)

	fmt.Fprintf(w, "SERVICE ID\t%s\n", svc.ServiceId)
	fmt.Fprintf(w, "NAME\t%s\n", svc.Name)
	fmt.Fprintf(w, "NAMESPACE\t%s\n", svc.Namespace)
	fmt.Fprintf(w, "SELECTOR\t%s\n", formatSelector(svc.Selector))
	fmt.Fprintf(w, "PORTS\t%s\n", formatPorts(svc.Ports))
	fmt.Fprintf(w, "CREATED AT\t%s\n", svc.CreatedAt)
	w.Flush()
}

func formatSelector(selector map[string]string) string {
	if len(selector) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(selector))
	for k := range selector {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, selector[k]))
	}
	return strings.Join(parts, ", ")
}
