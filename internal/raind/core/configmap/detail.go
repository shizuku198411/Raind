package configmap

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	httpclient "raind/internal/raind/core/client"
	"sort"
	"text/tabwriter"
)

func NewServiceDetail() *ServiceDetail {
	return &ServiceDetail{}
}

type ServiceDetail struct{}

func (s *ServiceDetail) Detail(idOrName, namespace string) error {
	if idOrName == "" {
		return fmt.Errorf("configmap id or name is required")
	}
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	path := "/v1/configmaps/" + url.PathEscape(idOrName)
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

func (s *ServiceDetail) printDetail(cm ConfigMapInfo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.DiscardEmptyColumns)
	fmt.Fprintf(w, "CONFIGMAP ID\t%s\n", cm.ConfigMapId)
	fmt.Fprintf(w, "NAME\t%s\n", cm.Name)
	fmt.Fprintf(w, "NAMESPACE\t%s\n", cm.Namespace)
	fmt.Fprintf(w, "CREATED AT\t%s\n", cm.CreatedAt)
	fmt.Fprintln(w, "DATA")
	keys := make([]string, 0, len(cm.Data))
	for k := range cm.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(w, "  %s\t%s\n", k, cm.Data[k])
	}
	w.Flush()
}
