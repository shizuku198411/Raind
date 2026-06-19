package secret

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

func NewServiceList() *ServiceList {
	return &ServiceList{}
}

type ServiceList struct{}

func (s *ServiceList) List(namespace string) error {
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	path := "/v1/secrets"
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
		if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		return fmt.Errorf("unexpected status: %s: %s", resp.Status, respModel.Message)
	}
	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	s.printList(respModel.Data)
	return nil
}

func (s *ServiceList) printList(list []SecretInfo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.DiscardEmptyColumns)
	fmt.Fprintln(w, "SECRET ID\tNAME\tNAMESPACE\tTYPE\tKEYS\tCREATED")
	for _, secret := range list {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n", secret.SecretId, secret.Name, secret.Namespace, secret.Type, len(secret.Keys), formatTime(secret.CreatedAt))
	}
	w.Flush()
}

func formatTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "less than a minutes"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	default:
		return t.Format("2006-01-02")
	}
}
