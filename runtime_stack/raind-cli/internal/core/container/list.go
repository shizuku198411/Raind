package container

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

func NewServiceContainerList() *ServiceContainerList {
	return &ServiceContainerList{}
}

type ServiceContainerList struct{}

func (s *ServiceContainerList) List() error {
	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return fmt.Errorf("sudo required")
	}
	httpClient.NewRequest(
		http.MethodGet,
		"/v1/containers",
		nil,
	)
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

	s.printContainerList(respModel.Data)

	return nil
}

func (s *ServiceContainerList) printContainerList(containerList []ContainerStateModel) {
	w := tabwriter.NewWriter(
		os.Stdout,
		0,
		0,
		2,
		' ',
		tabwriter.DiscardEmptyColumns,
	)

	// header
	fmt.Fprintln(w, "CONTAINER ID\tIMAGE\tCOMMAND\tCREATED\tSTATUS\tPORTS\tNAME")

	// helper: command formatter
	formatCommand := func(command []string) string {
		cmdStr := strings.Join(command, " ")
		if len(cmdStr) >= 20 {
			cmdStr = cmdStr[:20] + "…"
		}
		return fmt.Sprintf("\"%s\"", cmdStr)
	}

	// helper: time formatter
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

	// helper: port formatter
	formatPort := func(port []ForwardInfoModel) string {
		var tmp []string
		for _, p := range port {
			portStr := fmt.Sprintf("0.0.0.0:%d->%d/%s", p.HostPort, p.ContainerPort, p.Protocol)
			tmp = append(tmp, portStr)
		}
		return strings.Join(tmp, ",")
	}

	// data
	for _, c := range containerList {
		containerId := c.ContainerId
		image := strings.Split(c.Repository, "/")[1] + ":" + c.Reference
		command := formatCommand(c.Command)
		created := formatTime(c.CreatedAt)
		status := c.State
		port := formatPort(c.Forwards)
		name := c.Name
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", containerId, image, command, created, status, port, name)
	}

	w.Flush()
}
