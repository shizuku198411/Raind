package image

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	httpclient "raind/internal/core/client"
	"strings"
	"text/tabwriter"
	"time"
)

func NewServiceImageList() *ServiceImageList {
	return &ServiceImageList{}
}

type ServiceImageList struct{}

func (s *ServiceImageList) List() error {
	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return fmt.Errorf("sudo required")
	}
	httpClient.NewRequest(
		http.MethodGet,
		"/v1/images",
		nil,
	)
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	var respModel ImageListResponseModel

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

	s.printImageList(respModel.Data)

	return nil
}

func (s *ServiceImageList) printImageList(imageList []ImageDataModel) {
	w := tabwriter.NewWriter(
		os.Stdout,
		0,
		0,
		2,
		' ',
		tabwriter.DiscardEmptyColumns,
	)

	// header
	fmt.Fprintln(w, "REPOSITORY\tTAG\tCREATED")

	// helper: repository formatter
	formatRepository := func(repository string) string {
		parts := strings.Split(repository, "/")
		if parts[0] != "library" {
			return repository
		} else {
			return parts[1]
		}
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

	// data
	for _, i := range imageList {
		repository := formatRepository(i.Repository)
		tag := i.Reference
		created := formatTime(i.CreatedAt)
		fmt.Fprintf(w, "%s\t%s\t%s\n", repository, tag, created)
	}

	w.Flush()
}
