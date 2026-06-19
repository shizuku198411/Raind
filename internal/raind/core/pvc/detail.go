package pvc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	httpclient "raind/internal/raind/core/client"
	"strings"
	"text/tabwriter"
)

func NewServiceDetail() *ServiceDetail {
	return &ServiceDetail{}
}

type ServiceDetail struct{}

func (s *ServiceDetail) Detail(idOrName, namespace string) error {
	if idOrName == "" {
		return fmt.Errorf("persistentvolumeclaim id or name is required")
	}
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	path := "/v1/persistentvolumeclaims/" + url.PathEscape(idOrName)
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

func (s *ServiceDetail) printDetail(pvc PVCInfo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.DiscardEmptyColumns)
	fmt.Fprintf(w, "PVC ID\t%s\n", pvc.PVCId)
	fmt.Fprintf(w, "NAME\t%s\n", pvc.Name)
	fmt.Fprintf(w, "NAMESPACE\t%s\n", pvc.Namespace)
	fmt.Fprintf(w, "PHASE\t%s\n", pvc.Phase)
	fmt.Fprintf(w, "REQUESTED\t%s\n", pvc.RequestedStorage)
	fmt.Fprintf(w, "REQUESTED BYTES\t%d\n", pvc.RequestedBytes)
	fmt.Fprintf(w, "ACCESS MODES\t%s\n", strings.Join(pvc.AccessModes, ","))
	fmt.Fprintf(w, "RECLAIM POLICY\t%s\n", pvc.ReclaimPolicy)
	fmt.Fprintf(w, "DATA PATH\t%s\n", pvc.DataPath)
	fmt.Fprintf(w, "CREATED AT\t%s\n", pvc.CreatedAt)
	w.Flush()
}
