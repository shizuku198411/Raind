package networkpolicy

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
)

func NewServiceDetail() *ServiceDetail {
	return &ServiceDetail{}
}

type ServiceDetail struct{}

func (s *ServiceDetail) Detail(idOrName, namespace string) error {
	if idOrName == "" {
		return fmt.Errorf("networkpolicy id or name is required")
	}
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	path := "/v1/networkpolicies/" + url.PathEscape(idOrName)
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

	var respModel DetailResponseModel
	if !httpClient.IsStatusOk(resp) {
		if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		return fmt.Errorf("%s", respModel.Message)
	}
	if err := json.NewDecoder(resp.Body).Decode(&respModel); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	s.printDetail(respModel.Data)
	return nil
}

func (s *ServiceDetail) printDetail(np NetworkPolicyInfo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.DiscardEmptyColumns)
	fmt.Fprintf(w, "NETWORKPOLICY ID\t%s\n", np.NetworkPolicyId)
	fmt.Fprintf(w, "NAME\t%s\n", np.Name)
	fmt.Fprintf(w, "NAMESPACE\t%s\n", np.Namespace)
	fmt.Fprintf(w, "POD SELECTOR\t%s\n", labelsString(np.PodSelector))
	fmt.Fprintf(w, "INGRESS RULES\t%d\n", np.IngressRules)
	fmt.Fprintf(w, "EGRESS RULES\t%d\n", np.EgressRules)
	fmt.Fprintf(w, "GENERATED RULES\t%d\n", np.GeneratedRules)
	fmt.Fprintf(w, "CREATED AT\t%s\n", np.CreatedAt)
	w.Flush()
}

func labelsString(labels map[string]string) string {
	if len(labels) == 0 {
		return "<all>"
	}
	parts := make([]string, 0, len(labels))
	for key, value := range labels {
		parts = append(parts, key+"="+value)
	}
	sort.Strings(parts)
	return strings.Join(parts, ",")
}
