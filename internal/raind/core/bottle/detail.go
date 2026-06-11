package bottle

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	httpclient "raind/internal/raind/core/client"
	"raind/internal/raind/core/container"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

func NewServiceBottleDetail() *ServiceBottleDetail {
	return &ServiceBottleDetail{}
}

type ServiceBottleDetail struct{}

func (s *ServiceBottleDetail) Detail(target string) error {
	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return fmt.Errorf("sudo required")
	}
	httpClient.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/v1/bottle/%s", target),
		nil,
	)
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

	s.printBottleDetail(respModel.Data.Bottle)

	return nil
}

func (s *ServiceBottleDetail) printBottleDetail(b BottleDetailModel) {
	w := tabwriter.NewWriter(
		os.Stdout,
		0,
		0,
		2,
		' ',
		tabwriter.DiscardEmptyColumns,
	)

	fmt.Fprintf(w, "BOTTLE ID\t%s\n", b.BottleId)
	fmt.Fprintf(w, "BOTTLE NAME\t%s\n", b.BottleName)
	fmt.Fprintf(w, "CREATED AT\t%s\n", b.CreatedAt)
	fmt.Fprintf(w, "START ORDER\t%s\n", formatList(b.StartOrder))
	w.Flush()

	if len(b.Containers) > 0 {
		fmt.Println()
		s.printBottleContainers(b.Containers)
	}

	if len(b.Services) > 0 {
		fmt.Println()
		services := sortedServiceKeys(b.Services)
		for i, name := range services {
			svc := b.Services[name]
			command := svc.Command
			containerId := "-"
			image := svc.Image
			if containerInfo, ok := b.Containers[name]; ok {
				if len(containerInfo.Command) > 0 {
					command = containerInfo.Command
				}
				if containerInfo.ContainerId != "" {
					containerId = containerInfo.ContainerId
				}
				if containerInfo.Repository != "" && containerInfo.Reference != "" {
					image = fmt.Sprintf("%s:%s", containerInfo.Repository, containerInfo.Reference)
				}
			}
			w = tabwriter.NewWriter(
				os.Stdout,
				0,
				0,
				2,
				' ',
				tabwriter.DiscardEmptyColumns,
			)
			fmt.Fprintf(w, "SERVICE [%d]\t%s\n", i+1, name)
			fmt.Fprintf(w, "CONTAINER ID\t%s\n", containerId)
			fmt.Fprintf(w, "IMAGE\t%s\n", formatImage(image))
			fmt.Fprintf(w, "COMMAND\t%s\n", formatCommand(command))
			fmt.Fprintf(w, "ENV\t%s\n", formatList(svc.Env))
			fmt.Fprintf(w, "PORTS\t%s\n", formatList(svc.Ports))
			fmt.Fprintf(w, "MOUNT\t%s\n", formatList(svc.Mount))
			fmt.Fprintf(w, "NETWORK\t%s\n", emptyIfZero(svc.Network))
			fmt.Fprintf(w, "TTY\t%t\n", svc.Tty)
			fmt.Fprintf(w, "DEPENDS ON\t%s\n", formatList(svc.DependsOn))
			w.Flush()
			fmt.Println()
		}
	}

	if len(b.Policies) > 0 {
		w = tabwriter.NewWriter(
			os.Stdout,
			0,
			0,
			2,
			' ',
			tabwriter.DiscardEmptyColumns,
		)
		fmt.Fprintln(w, "POLICIES")
		fmt.Fprintln(w, "ID\tTYPE\tSOURCE\tDESTINATION\tPROTOCOL\tDPORT\tCOMMENT")
		for _, p := range b.Policies {
			dport := "-"
			if p.DestPort != 0 {
				dport = strconv.Itoa(p.DestPort)
			}
			fmt.Fprintf(
				w,
				"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				p.Id,
				p.Type,
				p.Source,
				p.Destination,
				p.Protocol,
				dport,
				p.Comment,
			)
		}
		w.Flush()
	}
}

func formatList(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, ", ")
}

func formatCommand(values []string) string {
	if len(values) == 0 {
		return "-"
	}
	return strings.Join(values, " ")
}

func emptyIfZero(v string) string {
	if v == "" {
		return "raind0"
	}
	return v
}

func formatImage(image string) string {
	if image == "" {
		return "-"
	}
	parts := strings.Split(image, "/")
	if len(parts) == 2 && parts[0] == "library" {
		return parts[1]
	}
	return image
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedServiceKeys(m map[string]BottleServiceModel) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (s *ServiceBottleDetail) printBottleContainers(containers map[string]container.ContainerStateModel) {
	w := tabwriter.NewWriter(
		os.Stdout,
		0,
		0,
		2,
		' ',
		tabwriter.DiscardEmptyColumns,
	)

	fmt.Fprintln(w, "SERVICES")
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
	formatPort := func(port []container.ForwardInfoModel) string {
		var tmp []string
		for _, p := range port {
			portStr := fmt.Sprintf("0.0.0.0:%d->%d/%s", p.HostPort, p.ContainerPort, p.Protocol)
			tmp = append(tmp, portStr)
		}
		return strings.Join(tmp, ",")
	}

	services := sortedContainerKeys(containers)
	for _, name := range services {
		c := containers[name]
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

func sortedContainerKeys(m map[string]container.ContainerStateModel) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
