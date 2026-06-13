package deployment

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

func NewServiceDeploymentList() *ServiceDeploymentList {
	return &ServiceDeploymentList{}
}

type ServiceDeploymentList struct{}

func (s *ServiceDeploymentList) List(namespace string) error {
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	path := "/v1/deployments"
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

	s.printDeploymentList(respModel.Data)
	return nil
}

func (s *ServiceDeploymentList) printDeploymentList(list []DeploymentInfoModel) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.DiscardEmptyColumns)
	fmt.Fprintln(w, "DEPLOYMENT ID\tNAME\tNAMESPACE\tDESIRED\tCURRENT\tREADY\tREPLICASET\tCREATED")

	for _, deploy := range list {
		fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%d\t%d\t%d\t%s\t%s\n",
			deploy.DeploymentId,
			deploy.Name,
			deploy.Namespace,
			deploy.Desired,
			deploy.Current,
			deploy.Ready,
			emptyIfZero(deploy.ReplicaSetId),
			formatTime(deploy.CreatedAt),
		)
	}

	w.Flush()
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
