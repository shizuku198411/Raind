package promote

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"raind/internal/raind/core/container"
)

const (
	defaultStrategyTimeout  = 60 * time.Second
	defaultStrategyInterval = 2 * time.Second
)

func RunStrategyCheck(check StrategyCheck) error {
	switch strings.ToLower(strings.TrimSpace(check.Type)) {
	case "", "http":
		return runHTTPStrategyCheck(check)
	case "tcp":
		return runTCPStrategyCheck(check)
	case "containerstatus", "container-status":
		return runContainerStatusStrategyCheck(check)
	case "bottlestatus", "bottle-status":
		return runBottleStatusStrategyCheck(check)
	default:
		return fmt.Errorf("unsupported check type %q", check.Type)
	}
}

func runHTTPStrategyCheck(check StrategyCheck) error {
	if strings.TrimSpace(check.Target) == "" {
		return fmt.Errorf("target is required")
	}
	var lastErr error
	deadline := time.Now().Add(checkTimeout(check))
	client := &http.Client{Timeout: checkInterval(check)}
	for {
		resp, err := client.Get(check.Target)
		if err != nil {
			lastErr = err
		} else {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
			resp.Body.Close()
			status := check.Expect.Status
			if status == 0 {
				status = http.StatusOK
			}
			if resp.StatusCode == status {
				if check.Expect.BodyContains == "" || strings.Contains(string(body), check.Expect.BodyContains) {
					return nil
				}
				lastErr = fmt.Errorf("response body does not contain %q", check.Expect.BodyContains)
			} else {
				lastErr = fmt.Errorf("unexpected status %d; expected %d", resp.StatusCode, status)
			}
		}
		if time.Now().After(deadline) {
			if lastErr == nil {
				lastErr = fmt.Errorf("timed out")
			}
			return lastErr
		}
		time.Sleep(checkInterval(check))
	}
}

func runTCPStrategyCheck(check StrategyCheck) error {
	if strings.TrimSpace(check.Target) == "" {
		return fmt.Errorf("target is required")
	}
	var lastErr error
	deadline := time.Now().Add(checkTimeout(check))
	for {
		conn, err := net.DialTimeout("tcp", check.Target, checkInterval(check))
		if err == nil {
			conn.Close()
			return nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return lastErr
		}
		time.Sleep(checkInterval(check))
	}
}

func runContainerStatusStrategyCheck(check StrategyCheck) error {
	if strings.TrimSpace(check.Target) == "" {
		return fmt.Errorf("target is required")
	}
	expected := strings.TrimSpace(check.Expect.State)
	if expected == "" {
		expected = "running"
	}
	var lastErr error
	deadline := time.Now().Add(checkTimeout(check))
	for {
		inspect, err := container.NewServiceContainerInspect().Get(check.Target)
		if err != nil {
			lastErr = err
		} else if strings.EqualFold(strings.TrimSpace(inspect.State), expected) {
			return nil
		} else {
			lastErr = fmt.Errorf("container state is %q; expected %q", inspect.State, expected)
		}
		if time.Now().After(deadline) {
			return lastErr
		}
		time.Sleep(checkInterval(check))
	}
}

func runBottleStatusStrategyCheck(check StrategyCheck) error {
	if strings.TrimSpace(check.Target) == "" {
		return fmt.Errorf("target is required")
	}
	var lastErr error
	deadline := time.Now().Add(checkTimeout(check))
	for {
		_, err := FetchRunningBottleDetail(check.Target)
		if err == nil {
			return nil
		}
		lastErr = err
		if time.Now().After(deadline) {
			return lastErr
		}
		time.Sleep(checkInterval(check))
	}
}

func checkTimeout(check StrategyCheck) time.Duration {
	if check.Timeout.Duration() <= 0 {
		return defaultStrategyTimeout
	}
	return check.Timeout.Duration()
}

func checkInterval(check StrategyCheck) time.Duration {
	if check.Interval.Duration() <= 0 {
		return defaultStrategyInterval
	}
	return check.Interval.Duration()
}

func checkName(check StrategyCheck) string {
	if strings.TrimSpace(check.Name) != "" {
		return strings.TrimSpace(check.Name)
	}
	if strings.TrimSpace(check.Target) != "" {
		return strings.TrimSpace(check.Target)
	}
	return strings.TrimSpace(check.Type)
}
