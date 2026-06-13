package namespace

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	httpclient "raind/internal/raind/core/client"
	"text/tabwriter"
)

func NewServiceNamespaceDetail() *ServiceNamespaceDetail {
	return &ServiceNamespaceDetail{}
}

type ServiceNamespaceDetail struct{}

func (s *ServiceNamespaceDetail) Detail(name string) error {
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	if err := httpClient.NewRequest(http.MethodGet, "/v1/namespaces/"+name, nil); err != nil {
		return err
	}
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel DetailResponseModel
	if !httpClient.IsStatusOk(resp) {
		if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		return fmt.Errorf("unexpected status: %s: %s", resp.Status, respModel.Message)
	}
	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	s.printNamespace(respModel.Data)
	return nil
}

func (s *ServiceNamespaceDetail) printNamespace(ns NamespaceInfoModel) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.DiscardEmptyColumns)
	fmt.Fprintf(w, "NAME\t%s\n", ns.Name)
	fmt.Fprintf(w, "NETWORK\t%s\n", ns.Network)
	fmt.Fprintf(w, "AUTO NETWORK\t%t\n", ns.NetworkAuto)
	fmt.Fprintf(w, "PODS\t%d\n", ns.Resources.Pods)
	fmt.Fprintf(w, "SERVICES\t%d\n", ns.Resources.Services)
	fmt.Fprintf(w, "REPLICASETS\t%d\n", ns.Resources.ReplicaSets)
	fmt.Fprintf(w, "DEPLOYMENTS\t%d\n", ns.Resources.Deployments)
	fmt.Fprintf(w, "ALLOCATIONS\t%d\n", ns.Resources.Allocations)
	fmt.Fprintf(w, "CREATED\t%s\n", formatNamespaceTime(ns.CreatedAt))
	w.Flush()
}
