package deployment

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

func NewServiceDeploymentDetail() *ServiceDeploymentDetail {
	return &ServiceDeploymentDetail{}
}

type ServiceDeploymentDetail struct{}

func (s *ServiceDeploymentDetail) Detail(id string) error {
	if id == "" {
		return fmt.Errorf("deployment id is required")
	}

	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	if err := httpClient.NewRequest(http.MethodGet, fmt.Sprintf("/v1/deployments/%s", id), nil); err != nil {
		return err
	}
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel DetailResponseModel
	if !httpClient.IsStatusOk(resp) {
		if decodeErr := json.NewDecoder(resp.Body).Decode(&respModel); decodeErr != nil {
			return fmt.Errorf("decode response: %w", decodeErr)
		}
		return fmt.Errorf("%s", respModel.Message)
	}
	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	s.printDeploymentDetail(respModel.Data)
	return nil
}

func (s *ServiceDeploymentDetail) printDeploymentDetail(deploy DeploymentDetailModel) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.DiscardEmptyColumns)
	fmt.Fprintf(w, "DEPLOYMENT ID\t%s\n", deploy.DeploymentId)
	fmt.Fprintf(w, "NAME\t%s\n", deploy.Name)
	fmt.Fprintf(w, "NAMESPACE\t%s\n", deploy.Namespace)
	fmt.Fprintf(w, "REPLICAS\t%d\n", deploy.Replicas)
	fmt.Fprintf(w, "CURRENT\t%d\n", deploy.Current)
	fmt.Fprintf(w, "READY\t%d\n", deploy.Ready)
	fmt.Fprintf(w, "REPLICASET\t%s\n", emptyIfZero(deploy.ReplicaSetId))
	fmt.Fprintf(w, "CREATED AT\t%s\n", deploy.CreatedAt)
	fmt.Fprintf(w, "UPDATED AT\t%s\n", deploy.UpdatedAt)
	w.Flush()

	if deploy.Template.Name == "" && deploy.Template.Namespace == "" && len(deploy.Template.Containers) == 0 {
		return
	}

	fmt.Println()
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.DiscardEmptyColumns)
	fmt.Fprintln(w, "TEMPLATE")
	fmt.Fprintf(w, "NAME\t%s\n", deploy.Template.Name)
	fmt.Fprintf(w, "NAMESPACE\t%s\n", deploy.Template.Namespace)
	fmt.Fprintf(w, "NETWORK NS\t%s\n", emptyIfZero(deploy.Template.NetworkNS))
	fmt.Fprintf(w, "IPC NS\t%s\n", emptyIfZero(deploy.Template.IpcNS))
	fmt.Fprintf(w, "UTS NS\t%s\n", emptyIfZero(deploy.Template.UtsNS))
	fmt.Fprintf(w, "USER NS\t%s\n", emptyIfZero(deploy.Template.UserNS))
	fmt.Fprintf(w, "LABELS\t%s\n", formatLabels(deploy.Template.Labels))
	w.Flush()

	if len(deploy.Template.Containers) == 0 {
		return
	}

	fmt.Println()
	w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.DiscardEmptyColumns)
	fmt.Fprintln(w, "CONTAINERS")
	fmt.Fprintln(w, "NAME\tIMAGE\tTTY")
	for _, c := range deploy.Template.Containers {
		fmt.Fprintf(w, "%s\t%s\t%t\n", c.Name, c.Image, c.Tty)
	}
	w.Flush()
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, labels[k]))
	}
	return strings.Join(parts, ", ")
}

func emptyIfZero(v string) string {
	if v == "" {
		return "-"
	}
	return v
}
