package namespace

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	httpclient "raind/internal/raind/core/client"
	"text/tabwriter"
	"time"
)

func NewServiceNamespaceList() *ServiceNamespaceList {
	return &ServiceNamespaceList{}
}

type ServiceNamespaceList struct{}

func (s *ServiceNamespaceList) List() error {
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	if err := httpClient.NewRequest(http.MethodGet, "/v1/namespaces", nil); err != nil {
		return err
	}
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel ListResponseModel
	if !httpClient.IsStatusOk(resp) {
		if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		return fmt.Errorf("unexpected status: %s: %s", resp.Status, respModel.Message)
	}
	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	s.printNamespaceList(respModel.Data)
	return nil
}

func (s *ServiceNamespaceList) printNamespaceList(namespaces []NamespaceInfoModel) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.DiscardEmptyColumns)
	fmt.Fprintln(w, "NAME\tNETWORK\tAUTO\tPODS\tSERVICES\tREPLICASETS\tDEPLOYMENTS\tALLOCATIONS\tCREATED")
	for _, ns := range namespaces {
		fmt.Fprintf(
			w,
			"%s\t%s\t%t\t%d\t%d\t%d\t%d\t%d\t%s\n",
			ns.Name,
			ns.Network,
			ns.NetworkAuto,
			ns.Resources.Pods,
			ns.Resources.Services,
			ns.Resources.ReplicaSets,
			ns.Resources.Deployments,
			ns.Resources.Allocations,
			formatNamespaceTime(ns.CreatedAt),
		)
	}
	w.Flush()
}

func formatNamespaceTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "less than a minutes"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}
