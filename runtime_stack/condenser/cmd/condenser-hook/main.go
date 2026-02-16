package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

func main() {
	var (
		u     = flag.String("url", "", "endpoint url (https://...)")
		event = flag.String("event", "", "hook event name (e.g. createRuntime)")
		ca    = flag.String("ca", "", "CA certificate path (pem)")
		cert  = flag.String("cert", "", "client certificate path (pem)")
		key   = flag.String("key", "", "client private key path (pem)")
	)
	flag.Parse()

	if *u == "" || *event == "" || *ca == "" || *cert == "" || *key == "" {
		fmt.Fprintln(os.Stderr, "required flags: --url --event --ca --cert --key")
		os.Exit(2)
	}

	// validate env
	if os.Getenv("RAIND-HOOK-SETTER") != "CONDENSER" {
		fmt.Fprintf(os.Stderr, "invalid raind-hook client\n")
		os.Exit(2)
	}

	// read stdin (OCI state.json)
	body, err := readBodyWithLimit(os.Stdin, 1<<20) // 1 MiB limit
	if err != nil {
		fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
		os.Exit(1)
	}

	switch *event {
	case "requestCert":
		_ = requestClientCertificate(body, u, ca, cert, key)
	case "createRuntime", "createContainer", "poststart", "stopContainer", "poststop":
		_ = postContainerState(body, event, u, ca, cert, key)
	default:
		fmt.Fprintf(os.Stderr, "invalid event: %s", *event)
		os.Exit(1)
	}

	os.Exit(0)
}

func readBodyWithLimit(r io.Reader, limit int64) ([]byte, error) {
	lr := &io.LimitedReader{R: r, N: limit + 1}
	b, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("input too large (>%d bytes)", limit)
	}
	return b, nil
}

func newMTLSClient(caPath, certPath, keyPath string) (*http.Client, error) {
	caPem, err := os.ReadFile(caPath)
	if err != nil {
		return nil, err
	}

	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caPem); !ok {
		return nil, errors.New("failed to append CA cert")
	}

	clientCert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}

	tlsCfg := &tls.Config{
		MinVersion:   tls.VersionTLS13,
		RootCAs:      pool,
		Certificates: []tls.Certificate{clientCert},
	}

	transport := &http.Transport{
		TLSClientConfig: tlsCfg,
	}

	return &http.Client{Transport: transport}, nil
}

func requestClientCertificate(body []byte, u *string, ca *string, cert *string, key *string) error {
	// generate csr
	var st State
	if err := json.Unmarshal(body, &st); err != nil {
		fmt.Fprintf(os.Stderr, "json parse: %v\n", err)
		os.Exit(1)
	}
	// if cert/key already exist, skip generate csr flow
	if !isCertificateExist(st) {
		if err := generateCsr(st); err != nil {
			fmt.Fprintf(os.Stderr, "generate csr: %v\n", err)
			os.Exit(1)
		}
		// request cert
		if err := requestCert(st, *u, *ca, *cert, *key); err != nil {
			fmt.Fprintf(os.Stderr, "request cert: %v\n", err)
			os.Exit(1)
		}
		// remove csr
		if err := removeCsr(st); err != nil {
			fmt.Fprintf(os.Stderr, "remove csr: %v\n", err)
			os.Exit(1)
		}
	}
	return nil
}

func postContainerState(body []byte, event *string, u *string, ca *string, cert *string, key *string) error {
	client, err := newMTLSClient(*ca, *cert, *key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init http client: %v\n", err)
		os.Exit(1)
	}

	req, err := http.NewRequest(http.MethodPost, *u, bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "new request: %v\n", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hook-Event", *event)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "post: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		fmt.Fprintf(os.Stderr, "post state failed")
		os.Exit(1)
	}

	return nil
}

type State struct {
	ContainerId string `json:"id"`
	Status      string `json:"status"`
}

func isCertificateExist(st State) bool {
	certPath := "/etc/raind/container/" + st.ContainerId + "/cert/client.key"
	keyPath := "/etc/raind/container/" + st.ContainerId + "/cert/client.key"

	_, certPathErr := os.Stat(certPath)
	_, keyPathErr := os.Stat(keyPath)
	return certPathErr == nil && keyPathErr == nil
}

func generateCsr(st State) error {
	spiffeId := "spiffe://raind/container/" + st.ContainerId
	csrPath := "/etc/raind/container/" + st.ContainerId + "/cert/req.csr"
	keyPath := "/etc/raind/container/" + st.ContainerId + "/cert/client.key"

	key, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return err
	}
	csrDer, err := buildCsrWithUriSAN(key, "raind-client", spiffeId)
	if err != nil {
		return err
	}
	if err := writePEM(keyPath, 0600, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(key)); err != nil {
		return err
	}
	if err := writePEM(csrPath, 0644, "CERTIFICATE REQUEST", csrDer); err != nil {
		return err
	}
	return nil
}

func requestCert(st State, url string, ca string, cert string, key string) error {
	csrPem, _ := os.ReadFile("/etc/raind/container/" + st.ContainerId + "/cert/req.csr")

	client, err := newMTLSClient(ca, cert, key)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(
		http.MethodPost,
		url,
		bytes.NewReader(csrPem))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/pem-certificate-chain")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return err
	}

	certPem, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	err = os.WriteFile("/etc/raind/container/"+st.ContainerId+"/cert/client.crt", certPem, 0644)
	if err != nil {
		return err
	}
	return nil
}

func removeCsr(st State) error {
	return os.Remove("/etc/raind/container/" + st.ContainerId + "/cert/req.csr")
}

func buildCsrWithUriSAN(key *rsa.PrivateKey, cn string, spiffe string) ([]byte, error) {
	uri, err := url.Parse(spiffe)
	if err != nil || uri.Scheme != "spiffe" || uri.Host == "" {
		return nil, fmt.Errorf("invalid spiffe id: %q", spiffe)
	}

	tql := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: cn,
		},
	}
	tql.URIs = []*url.URL{uri}

	csrDer, err := x509.CreateCertificateRequest(rand.Reader, tql, key)
	if err != nil {
		return nil, err
	}

	return csrDer, nil
}

func writePEM(path string, perm os.FileMode, typ string, der []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	defer f.Close()

	return pem.Encode(f, &pem.Block{Type: typ, Bytes: der})
}
