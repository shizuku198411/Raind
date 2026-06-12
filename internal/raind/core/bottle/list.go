package bottle

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	httpclient "raind/internal/raind/core/client"
	"text/tabwriter"
)

func NewServiceBottleList() *ServiceBottleList {
	return &ServiceBottleList{}
}

type ServiceBottleList struct{}

func (s *ServiceBottleList) List() error {
	httpClient, err := httpclient.NewHttpClient()
	if err != nil {
		return err
	}
	httpClient.NewRequest(
		http.MethodGet,
		"/v1/bottle",
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

	s.printBottleList(respModel.Data.Bottles)

	return nil
}

func (s *ServiceBottleList) printBottleList(bottles []BottleListItemModel) {
	w := tabwriter.NewWriter(
		os.Stdout,
		0,
		0,
		2,
		' ',
		tabwriter.DiscardEmptyColumns,
	)

	fmt.Fprintln(w, "BOTTLE ID\tBOTTLE NAME\tSERVICES\tSTATUS")

	for _, b := range bottles {
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", b.BottleId, b.BottleName, b.ServiceCount, b.Status)
	}

	w.Flush()
}
