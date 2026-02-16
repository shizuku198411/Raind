package replicaset

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	httpclient "raind/internal/core/client"
	"text/tabwriter"
	"time"
)

func NewServiceReplicaSetList() *ServiceReplicaSetList {
	return &ServiceReplicaSetList{}
}

type ServiceReplicaSetList struct{}

func (s *ServiceReplicaSetList) List() error {
	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return fmt.Errorf("sudo required")
	}
	if err := httpClient.NewRequest(
		http.MethodGet,
		"/v1/replicasets",
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

	s.printReplicaSetList(respModel.Data)
	return nil
}

func (s *ServiceReplicaSetList) printReplicaSetList(list []ReplicaSetInfoModel) {
	w := tabwriter.NewWriter(
		os.Stdout,
		0,
		0,
		2,
		' ',
		tabwriter.DiscardEmptyColumns,
	)

	fmt.Fprintln(w, "REPLICASET ID\tNAME\tNAMESPACE\tDESIRED\tCURRENT\tREADY\tCREATED")

	formatTime := func(t time.Time) string {
		now := time.Now()
		d := now.Sub(t)

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

	for _, rs := range list {
		created := formatTime(rs.CreatedAt)
		fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%d\t%d\t%d\t%s\n",
			rs.ReplicaSetId,
			rs.Name,
			rs.Namespace,
			rs.Desired,
			rs.Current,
			rs.Ready,
			created,
		)
	}

	w.Flush()
}
