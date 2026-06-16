package securityprofile

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	httpclient "raind/internal/raind/core/client"
	"text/tabwriter"
)

func NewServiceList() *ServiceList {
	return &ServiceList{}
}

type ServiceList struct{}

func (s *ServiceList) List() error {
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	if err := httpClient.NewRequest(http.MethodGet, "/v1/security/profiles", nil); err != nil {
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

	return s.print(respModel.Data.Profiles)
}

func (s *ServiceList) print(profiles []ProfileSummary) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.DiscardEmptyColumns)
	fmt.Fprintln(w, "NAME\tTYPE\tCAPABILITIES\tSECCOMP\tAPPARMOR")
	for _, profile := range profiles {
		seccomp := "disabled"
		if profile.SeccompEnabled {
			seccomp = "enabled"
		}
		apparmor := profile.AppArmorProfile
		if apparmor == "" {
			apparmor = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%d caps\t%s\t%s\n", profile.Name, profile.Type, profile.CapabilitiesCount, seccomp, apparmor)
	}
	return w.Flush()
}
