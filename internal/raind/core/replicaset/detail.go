package replicaset

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

func NewServiceReplicaSetDetail() *ServiceReplicaSetDetail {
	return &ServiceReplicaSetDetail{}
}

type ServiceReplicaSetDetail struct{}

func (s *ServiceReplicaSetDetail) Detail(id string) error {
	if id == "" {
		return fmt.Errorf("replicaset id is required")
	}

	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return fmt.Errorf("sudo required")
	}
	if err := httpClient.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/v1/replicasets/%s", id),
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

	s.printReplicaSetDetail(respModel.Data)
	return nil
}

func (s *ServiceReplicaSetDetail) printReplicaSetDetail(rs ReplicaSetDetailModel) {
	w := tabwriter.NewWriter(
		os.Stdout,
		0,
		0,
		2,
		' ',
		tabwriter.DiscardEmptyColumns,
	)

	fmt.Fprintf(w, "REPLICASET ID\t%s\n", rs.ReplicaSetId)
	fmt.Fprintf(w, "NAME\t%s\n", rs.Name)
	fmt.Fprintf(w, "NAMESPACE\t%s\n", rs.Namespace)
	fmt.Fprintf(w, "REPLICAS\t%d\n", rs.Replicas)
	fmt.Fprintf(w, "CREATED AT\t%s\n", rs.CreatedAt)
	w.Flush()

	if rs.Template.Name == "" && rs.Template.Namespace == "" && len(rs.Template.Containers) == 0 {
		return
	}

	fmt.Println()
	w = tabwriter.NewWriter(
		os.Stdout,
		0,
		0,
		2,
		' ',
		tabwriter.DiscardEmptyColumns,
	)

	fmt.Fprintln(w, "TEMPLATE")
	fmt.Fprintf(w, "NAME\t%s\n", rs.Template.Name)
	fmt.Fprintf(w, "NAMESPACE\t%s\n", rs.Template.Namespace)
	fmt.Fprintf(w, "NETWORK NS\t%s\n", emptyIfZero(rs.Template.NetworkNS))
	fmt.Fprintf(w, "IPC NS\t%s\n", emptyIfZero(rs.Template.IpcNS))
	fmt.Fprintf(w, "UTS NS\t%s\n", emptyIfZero(rs.Template.UtsNS))
	fmt.Fprintf(w, "USER NS\t%s\n", emptyIfZero(rs.Template.UserNS))
	fmt.Fprintf(w, "LABELS\t%s\n", formatLabels(rs.Template.Labels))
	w.Flush()

	if len(rs.Template.Containers) == 0 {
		return
	}

	fmt.Println()
	w = tabwriter.NewWriter(
		os.Stdout,
		0,
		0,
		2,
		' ',
		tabwriter.DiscardEmptyColumns,
	)
	fmt.Fprintln(w, "CONTAINERS")
	fmt.Fprintln(w, "NAME\tIMAGE\tTTY")
	for _, c := range rs.Template.Containers {
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
