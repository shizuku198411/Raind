package httpclient

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
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
