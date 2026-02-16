package container

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	httpclient "raind/internal/core/client"
)

func NewServiceContainerLog() *ServiceContainerLog {
	return &ServiceContainerLog{}
}

type ServiceContainerLog struct{}

func (s *ServiceContainerLog) GetLog(param ServiceLogsModel) error {
	var tailLaine int
	if param.TailLine <= 0 {
		if param.Pager {
			tailLaine = 5000
		} else {
			tailLaine = 100
		}
	} else {
		tailLaine = param.TailLine
	}
	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return fmt.Errorf("sudo required")
	}

	reqUrl := fmt.Sprintf("/v1/containers/%s/log?tail_lines=%d", param.ContainerId, tailLaine)

	httpClient.NewRequest(
		http.MethodGet,
		reqUrl,
		nil,
	)
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	if !httpClient.IsStatusOk(resp) {
		var respModel LogsResponseModel
		err := json.NewDecoder(resp.Body).Decode(&respModel)
		if err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		return fmt.Errorf(respModel.Message)
	}

	data, _ := io.ReadAll(resp.Body)

	// print
	if param.Pager {
		return s.openWithPager(data)
	} else {
		return s.printToStdout(data)
	}
}

func (s *ServiceContainerLog) printToStdout(data []byte) error {
	_, err := os.Stdout.Write(data)
	return err
}

func (s *ServiceContainerLog) openWithPager(data []byte) error {
	tmp, err := os.CreateTemp("", "raind-log-*.log")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if _, err := exec.LookPath("less"); err != nil {
		// less not found, fallback to printToStdout
		return s.printToStdout(data)
	}
	cmd := exec.Command("less", "-R", tmp.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
