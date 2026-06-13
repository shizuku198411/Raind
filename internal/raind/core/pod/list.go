package pod

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	httpclient "raind/internal/raind/core/client"
	"text/tabwriter"
	"time"
)

func NewServicePodList() *ServicePodList {
	return &ServicePodList{}
}

type ServicePodList struct{}

func (s *ServicePodList) List(namespace string) error {
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	path := "/v1/pods"
	if namespace != "" {
		path += "?namespace=" + url.QueryEscape(namespace)
	}
	if err := httpClient.NewRequest(
		http.MethodGet,
		path,
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

	s.printPodList(respModel.Data)

	return nil
}

func (s *ServicePodList) printPodList(pods []PodInfoModel) {
	w := tabwriter.NewWriter(
		os.Stdout,
		0,
		0,
		2,
		' ',
		tabwriter.DiscardEmptyColumns,
	)

	fmt.Fprintln(w, "POD ID\tNAME\tNAMESPACE\tUID\tREADY\tSTATE\tCREATED")

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

	for _, p := range pods {
		created := formatTime(p.CreatedAt)
		fmt.Fprintf(w, "%s\t%s\t%s\t%d/%d\t%s\t%s\n", p.PodId, p.Name, p.Namespace, p.RunningContainers, p.DesiredContainers, p.State, created)
	}

	w.Flush()
}
