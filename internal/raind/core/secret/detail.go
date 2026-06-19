package secret

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
		return fmt.Errorf("secret id or name is required")
	}
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	path := "/v1/secrets/" + url.PathEscape(idOrName)
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

func (s *ServiceDetail) printDetail(secret SecretInfo) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.DiscardEmptyColumns)
	fmt.Fprintf(w, "SECRET ID\t%s\n", secret.SecretId)
	fmt.Fprintf(w, "NAME\t%s\n", secret.Name)
	fmt.Fprintf(w, "NAMESPACE\t%s\n", secret.Namespace)
	fmt.Fprintf(w, "TYPE\t%s\n", secret.Type)
	fmt.Fprintf(w, "KEYS\t%s\n", strings.Join(secret.Keys, ","))
	fmt.Fprintf(w, "CREATED AT\t%s\n", secret.CreatedAt)
	w.Flush()
}
