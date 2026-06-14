package ingress

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	httpclient "raind/internal/raind/core/client"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

func NewServiceIngressList() *ServiceIngressList {
	return &ServiceIngressList{}
}

type ServiceIngressList struct{}

func (s *ServiceIngressList) List(namespace string) error {
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	path := "/v1/ingresses"
	if namespace != "" {
		path += "?namespace=" + url.QueryEscape(namespace)
	}
	if err := httpClient.NewRequest(http.MethodGet, path, nil); err != nil {
		return err
	}
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel ListResponseModel
	if !httpClient.IsStatusOk(resp) {
		if decodeErr := json.NewDecoder(resp.Body).Decode(&respModel); decodeErr != nil {
			return fmt.Errorf("decode response: %w", decodeErr)
		}
		return fmt.Errorf("unexpected status: %s: %s", resp.Status, respModel.Message)
	}
	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	s.printIngressList(respModel.Data)
	return nil
}

func (s *ServiceIngressList) printIngressList(list []IngressInfoModel) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.DiscardEmptyColumns)
	fmt.Fprintf(w, "INGRESS ID\tNAME\tNAMESPACE\tHOSTS\tPATHS\tBACKENDS\tCREATED\n")

	for _, in := range list {
		fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			in.IngressId,
			in.Name,
			in.Namespace,
			formatHosts(in.Rules),
			formatPaths(in.Rules),
			formatBackends(in.Rules),
			formatTime(in.CreatedAt),
		)
	}
	w.Flush()
}

func formatHosts(rules []IngressRuleModel) string {
	if len(rules) == 0 {
		return "-"
	}
	seen := map[string]bool{}
	var hosts []string
	for _, r := range rules {
		host := strings.TrimSpace(r.Host)
		if host == "" {
			host = "*"
		}
		if !seen[host] {
			seen[host] = true
			hosts = append(hosts, host)
		}
	}
	sort.Strings(hosts)
	return strings.Join(hosts, ",")
}

func formatPaths(rules []IngressRuleModel) string {
	var paths []string
	for _, r := range rules {
		host := strings.TrimSpace(r.Host)
		if host == "" {
			host = "*"
		}
		for _, p := range r.Paths {
			path := p.Path
			if path == "" {
				path = "/"
			}
			pathType := p.PathType
			if pathType == "" {
				pathType = "Prefix"
			}
			paths = append(paths, fmt.Sprintf("%s%s(%s)", host, path, pathType))
		}
	}
	if len(paths) == 0 {
		return "-"
	}
	return strings.Join(paths, ",")
}

func formatBackends(rules []IngressRuleModel) string {
	var backends []string
	for _, r := range rules {
		for _, p := range r.Paths {
			if p.Backend.ServiceName == "" || p.Backend.ServicePort == 0 {
				continue
			}
			backends = append(backends, fmt.Sprintf("%s:%d", p.Backend.ServiceName, p.Backend.ServicePort))
		}
	}
	if len(backends) == 0 {
		return "-"
	}
	return strings.Join(backends, ",")
}

func formatTime(t time.Time) string {
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
