package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"raind/internal/raind-ui-gateway/buildinfo"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	defaultSocketPath   = "/run/raind/ui.sock"
	defaultSocketMode   = "0660"
	defaultCondenserURL = "https://127.0.0.1:7755"
	defaultCACertPath   = "/etc/raind/cert/raind.crt"
	defaultClientCert   = "/etc/raind/cert/raindWebClient.crt"
	defaultClientKey    = "/etc/raind/cert/raindWebClient.key"
)

type config struct {
	socketPath   string
	socketMode   os.FileMode
	condenserURL *url.URL
	caCertPath   string
	clientCert   string
	clientKey    string
}

func main() {
	showVersion := flag.Bool("version", false, "show version")
	flag.Parse()
	if *showVersion {
		fmt.Println(buildinfo.String())
		return
	}

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	client, err := newMTLSClient(cfg)
	if err != nil {
		log.Fatalf("http client init failed: %v", err)
	}

	if err := prepareSocket(cfg.socketPath); err != nil {
		log.Fatalf("socket prepare failed: %v", err)
	}

	ln, err := net.Listen("unix", cfg.socketPath)
	if err != nil {
		log.Fatalf("listen unix socket failed: %v", err)
	}
	defer ln.Close()

	if err := os.Chmod(cfg.socketPath, cfg.socketMode); err != nil {
		log.Fatalf("chmod socket failed: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(cfg.condenserURL)
	proxy.Transport = client.Transport
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		http.Error(w, "upstream request failed", http.StatusBadGateway)
	}
	h := &proxyHandler{
		proxy: proxy,
	}

	srv := &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("raind-ui-gateway listening on %s", cfg.socketPath)
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func loadConfig() (config, error) {
	socketMode, err := parseFileMode(getenv("RAIND_UI_SOCKET_MODE", defaultSocketMode))
	if err != nil {
		return config{}, err
	}
	condenserRaw := getenv("RAIND_UI_CONDENSER_URL", defaultCondenserURL)
	cu, err := url.Parse(condenserRaw)
	if err != nil {
		return config{}, fmt.Errorf("parse condenser url: %w", err)
	}
	if cu.Scheme != "https" {
		return config{}, fmt.Errorf("condenser url must use https")
	}
	return config{
		socketPath:   getenv("RAIND_UI_SOCKET_PATH", defaultSocketPath),
		socketMode:   socketMode,
		condenserURL: cu,
		caCertPath:   getenv("RAIND_UI_CA_CERT", defaultCACertPath),
		clientCert:   getenv("RAIND_UI_CLIENT_CERT", defaultClientCert),
		clientKey:    getenv("RAIND_UI_CLIENT_KEY", defaultClientKey),
	}, nil
}

func newMTLSClient(cfg config) (*http.Client, error) {
	caPEM, err := os.ReadFile(cfg.caCertPath)
	if err != nil {
		return nil, fmt.Errorf("read ca cert: %w", err)
	}
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caPEM); !ok {
		return nil, fmt.Errorf("append ca cert failed")
	}
	cert, err := tls.LoadX509KeyPair(cfg.clientCert, cfg.clientKey)
	if err != nil {
		return nil, fmt.Errorf("load client cert/key: %w", err)
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:   tls.VersionTLS13,
			RootCAs:      pool,
			Certificates: []tls.Certificate{cert},
		},
	}
	return &http.Client{Transport: tr, Timeout: 60 * time.Second}, nil
}

func parseFileMode(v string) (os.FileMode, error) {
	n, err := strconv.ParseUint(v, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid socket mode %q: %w", v, err)
	}
	return os.FileMode(n), nil
}

func prepareSocket(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return fmt.Errorf("path exists and is not a unix socket: %s", path)
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		return nil
	}
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func getenv(k, fallback string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return fallback
	}
	return v
}

type proxyHandler struct {
	proxy *httputil.ReverseProxy
}

func (h *proxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok\n")
		return
	}

	if !isPathAllowed(r.URL.Path) {
		http.Error(w, "forbidden path", http.StatusForbidden)
		return
	}
	if !isMethodAllowed(r.Method) {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.proxy.ServeHTTP(w, r)
}

func isMethodAllowed(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodDelete:
		return true
	default:
		return false
	}
}

func isPathAllowed(path string) bool {
	return strings.HasPrefix(path, "/v1/")
}
