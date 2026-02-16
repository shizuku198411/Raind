package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	httpclient "raind/internal/core/client"
	"strings"
	"text/tabwriter"
	"time"
)

func NewServiceServiceList() *ServiceServiceList {
	return &ServiceServiceList{}
}

type ServiceServiceList struct{}

func (s *ServiceServiceList) List() error {
	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return fmt.Errorf("sudo required")
	}
	if err := httpClient.NewRequest(
		http.MethodGet,
		"/v1/services",
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

	s.printServiceList(respModel.Data)
	return nil
}

func (s *ServiceServiceList) printServiceList(list []ServiceInfoModel) {
	w := tabwriter.NewWriter(
		os.Stdout,
		0,
		0,
		2,
		' ',
		tabwriter.DiscardEmptyColumns,
	)

	fmt.Fprintln(w, "SERVICE ID\tNAME\tNAMESPACE\tPORTS\tCREATED")

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

	for _, svc := range list {
		created := formatTime(svc.CreatedAt)
		ports := formatPorts(svc.Ports)
		fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%s\t%s\n",
			svc.ServiceId,
			svc.Name,
			svc.Namespace,
			ports,
			created,
		)
	}

	w.Flush()
}

func formatPorts(ports []ServicePortModel) string {
	if len(ports) == 0 {
		return "-"
	}
	items := make([]string, 0, len(ports))
	for _, p := range ports {
		proto := strings.ToLower(p.Protocol)
		if proto == "" {
			proto = "tcp"
		}
		items = append(items, fmt.Sprintf("%d->%d/%s", p.Port, p.TargetPort, proto))
	}
	return strings.Join(items, ",")
}
