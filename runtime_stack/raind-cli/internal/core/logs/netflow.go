package logs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	httpclient "raind/internal/core/client"
	"strings"
)

func NewServiceNetflowLog() *ServiceNetflowLog {
	return &ServiceNetflowLog{}
}

type ServiceNetflowLog struct{}

func (s *ServiceNetflowLog) GetLoge(param ServiceNetflowModel) error {
	var tailLine int
	if param.TailLine <= 0 {
		if param.Pager {
			tailLine = 500
		} else {
			tailLine = 100
		}
	} else {
		tailLine = param.TailLine
	}

	httpClient := httpclient.NewHttpClient()
	if httpClient == nil {
		return fmt.Errorf("sudo required")
	}
	httpClient.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/v1/logs/netflow?tail_lines=%d", tailLine),
		nil,
	)
	resp, err := httpClient.Client.Do(httpClient.Request)
	if err != nil {
		return fmt.Errorf("Cannot connect to the Raind daemon. Is the raind daemon running?")
	}
	defer resp.Body.Close()

	if !httpClient.IsStatusOk(resp) {
		var respModel NetflowResponseModel
		err := json.NewDecoder(resp.Body).Decode(&respModel)
		if err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		return fmt.Errorf(respModel.Message)
	}

	data, _ := io.ReadAll(resp.Body)

	if param.Pager {
		if param.JsonView {
			return s.openWithPager(data)
		} else {
			lines, err := s.formatAndFilterNetflow(data)
			if err != nil {
				return err
			}
			out := strings.Join(lines, "\n") + "\n"
			return s.openWithPager([]byte(out))
		}
	} else {
		if param.JsonView {
			return s.printToStdout(data)
		} else {
			lines, err := s.formatAndFilterNetflow(data)
			if err != nil {
				return err
			}
			out := strings.Join(lines, "\n") + "\n"
			return s.printToStdout([]byte(out))
		}
	}
}

func (s *ServiceNetflowLog) printToStdout(data []byte) error {
	_, err := os.Stdout.Write(data)
	return err
}

func (s *ServiceNetflowLog) openWithPager(data []byte) error {
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

func (s *ServiceNetflowLog) formatAndFilterNetflow(jsonl []byte) ([]string, error) {
	sc := bufio.NewScanner(bytes.NewReader(jsonl))
	const maxLine = 4 * 1024 * 1024
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, maxLine)
	out := make([]string, 0, 256)

	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var lf NetflowLog
		if err := json.Unmarshal(line, &lf); err != nil {
			continue
		}
		out = append(out, s.formatNetflowLogLine(&lf))
	}

	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *ServiceNetflowLog) formatNetflowLogLine(l *NetflowLog) string {
	ts := l.ReveivedTS
	if ts == "" {
		ts = l.GeneratedTS
	}
	ts = s.trimRFC3339ForHuman(ts)

	verdict := strings.ToUpper(l.Verdict)

	from := l.Src
	fromLabel := s.endpointLabel(from)
	to := l.Dst
	toLabel := s.endpointLabel(to)

	protoPart := strings.ToUpper(l.Proto)
	detail := ""
	switch protoPart {
	case "TCP", "UDP":
		if to.Port != 0 {
			detail = fmt.Sprintf("{%s/%d}", protoPart, to.Port)
		} else if from.Port != 0 {
			detail = fmt.Sprintf("{%s/%d}", protoPart, from.Port)
		} else {
			detail = fmt.Sprintf("{%s}", protoPart)
		}
	case "ICMP":
		if l.Icmp != nil {
			detail = fmt.Sprintf("{ICMP type=%d code=%d}", l.Icmp.Type, l.Icmp.Code)
		} else {
			detail = "{ICMP}"
		}
	default:
		detail = fmt.Sprintf("{%s}", protoPart)
	}

	return fmt.Sprintf("%s\t%s\tFROM: %s => TO: %s %s", ts, verdict, fromLabel, toLabel, detail)
}

func (s *ServiceNetflowLog) endpointLabel(e Endpoint) string {
	if e.Kind == "container" {
		if e.ContainerName != "" && e.ContainerName != "container_unresolved" {
			return e.ContainerName
		}
		if e.ContainerId != "" {
			return e.ContainerId
		}
	}
	if e.IP != "" {
		return e.IP
	}
	return "unknown"
}

func (s *ServiceNetflowLog) trimRFC3339ForHuman(str string) string {
	str = strings.ReplaceAll(str, "T", " ")
	if i := strings.IndexByte(str, '.'); i >= 0 {
		str = str[:i]
	}
	if i := strings.Index(str, "+"); i >= 0 {
		str = str[:i]
	}
	if i := strings.Index(str, "Z"); i >= 0 {
		str = str[:i]
	}
	return str
}
