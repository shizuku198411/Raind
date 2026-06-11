package httpclient

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"raind/internal/raind/utils"

	"github.com/gorilla/websocket"
)

func NewHttpClient() *HttpClient {
	certPool := x509.NewCertPool()
	pemBytes, err := os.ReadFile(utils.PublicCertPath)
	if err != nil {
		return nil
	}

	if ok := certPool.AppendCertsFromPEM(pemBytes); !ok {
		return nil
	}

	clientCert, err := tls.LoadX509KeyPair(utils.ClientCertPath, utils.ClientKeyPath)
	if err != nil {
		return nil
	}
	return &HttpClient{
		BaseUrl: "https://localhost:7755",
		Client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs:      certPool,
					Certificates: []tls.Certificate{clientCert},
				},
			},
		},
	}
}

type HttpClient struct {
	BaseUrl string
	Client  *http.Client
	Request *http.Request
}

type StreamEvent struct {
	Status  string          `json:"status"`
	ID      string          `json:"id,omitempty"`
	Detail  string          `json:"detail,omitempty"`
	Current int64           `json:"current,omitempty"`
	Total   int64           `json:"total,omitempty"`
	Error   string          `json:"error,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (c *HttpClient) NewRequest(method string, path string, body []byte) error {
	var err error
	if method == http.MethodPost ||
		method == http.MethodPut ||
		method == http.MethodPatch ||
		method == http.MethodDelete {
		c.Request, err = http.NewRequest(
			method,
			c.BaseUrl+path,
			bytes.NewReader(body),
		)
	} else {
		c.Request, err = http.NewRequest(
			method,
			c.BaseUrl+path,
			nil,
		)
	}
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	c.Request.Header.Set("Content-Type", "application/json")
	return nil
}

func (c *HttpClient) IsStatusOk(resp *http.Response) bool {
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return false
	}
	return true
}

func ReadStreamEvents(r io.Reader) (StreamEvent, error) {
	dec := json.NewDecoder(r)
	var last StreamEvent
	printer := newStreamPrinter()
	defer printer.finish()
	for {
		var event StreamEvent
		if err := dec.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				return last, nil
			}
			return last, err
		}
		last = event
		if event.Status == "error" {
			return last, errors.New(event.Error)
		}
		printer.print(event)
	}
	return last, nil
}

type streamPrinter struct {
	activeRewrite bool
	rewriteID     string
}

func newStreamPrinter() *streamPrinter {
	return &streamPrinter{}
}

func (p *streamPrinter) print(event StreamEvent) {
	if event.Status == "success" || event.Status == "done" {
		p.finish()
		if event.Detail != "" {
			fmt.Println(event.Detail)
		}
		return
	}
	if event.Status == "downloading" && event.Current == 0 {
		return
	}

	line := formatStreamEvent(event)
	if event.Status == "complete" && p.activeRewrite && p.rewriteID == event.ID {
		p.rewrite(line, event.ID)
		p.finish()
		return
	}
	if shouldRewriteStreamEvent(event) {
		p.rewrite(line, event.ID)
		return
	}

	p.finish()
	fmt.Println(line)
}

func (p *streamPrinter) rewrite(line string, id string) {
	if p.activeRewrite && p.rewriteID != id {
		fmt.Println()
	}
	p.activeRewrite = true
	p.rewriteID = id
	fmt.Printf("\r\033[K%s", line)
}

func (p *streamPrinter) finish() {
	if p.activeRewrite {
		fmt.Println()
		p.activeRewrite = false
		p.rewriteID = ""
	}
}

func shouldRewriteStreamEvent(event StreamEvent) bool {
	return event.Status == "downloading" && event.ID != "" && event.Total > 0 && event.Current > 0
}

func formatStreamEvent(event StreamEvent) string {
	label := event.ID
	if label == "" {
		label = event.Status
	}
	detail := event.Detail
	if detail == "" {
		detail = event.Status
	}
	if event.Total > 0 {
		if event.Status == "extracting" {
			return fmt.Sprintf("%-16s %s %d/%d", label, detail, event.Current, event.Total)
		}
		if event.Current > 0 {
			return fmt.Sprintf("%-16s %s %s/%s", label, detail, formatBytes(event.Current), formatBytes(event.Total))
		}
	}
	return fmt.Sprintf("%-16s %s", label, detail)
}

func formatBytes(v int64) string {
	const unit = 1024
	if v < unit {
		return fmt.Sprintf("%dB", v)
	}
	div, exp := int64(unit), 0
	for n := v / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%ciB", float64(v)/float64(div), "KMGTPE"[exp])
}

func (c *HttpClient) NewMTLSDialer(caPath, clientCertPath, clientKeyPath string) (*websocket.Dialer, error) {
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return nil, err
	}
	rootPool := x509.NewCertPool()
	if ok := rootPool.AppendCertsFromPEM(caPEM); !ok {
		return nil, errors.New("failed to append CA")
	}

	clientCert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
	if err != nil {
		return nil, err
	}

	d := *websocket.DefaultDialer
	d.TLSClientConfig = &tls.Config{
		MinVersion:   tls.VersionTLS13,
		RootCAs:      rootPool,
		Certificates: []tls.Certificate{clientCert},
	}

	return &d, nil
}
