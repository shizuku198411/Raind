package ingress

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"raind/internal/condenser/core/cert"
	"raind/internal/condenser/utils"
)

func NewTLSManager() *TLSManager {
	return &TLSManager{certHandler: cert.NewCertManager()}
}

type TLSManager struct {
	certHandler cert.CertHandler
}

func (m *TLSManager) EnsureHosts(hosts []string) error {
	for _, host := range normalizeTLSHosts(hosts) {
		if err := m.EnsureHost(host); err != nil {
			return err
		}
	}
	return nil
}

func (m *TLSManager) RemoveHostsIfUnused(hosts []string, activeHosts []string) error {
	active := map[string]bool{}
	for _, host := range normalizeTLSHosts(activeHosts) {
		active[host] = true
	}
	for _, host := range normalizeTLSHosts(hosts) {
		if active[host] {
			continue
		}
		if err := m.RemoveHost(host); err != nil {
			return err
		}
	}
	return nil
}

func (m *TLSManager) RemoveHost(host string) error {
	host = strings.ToLower(strings.TrimSpace(host))
	if err := validateTLSHost(host); err != nil {
		return err
	}
	certPath, _, err := ingressCertPaths(host)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(filepath.Dir(certPath)); err != nil {
		return err
	}
	return nil
}

func (m *TLSManager) EnsureHost(host string) error {
	host = strings.ToLower(strings.TrimSpace(host))
	if err := validateTLSHost(host); err != nil {
		return err
	}
	certPath, keyPath, err := ingressCertPaths(host)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(certPath), 0o700); err != nil {
		return err
	}
	return m.certHandler.IssueServerCert(
		certPath,
		keyPath,
		utils.IngressIssuerCACertPath,
		utils.IngressIssuerCAKeyPath,
		cert.ServerCertConfig{
			CommonName:  host,
			DNSNames:    []string{host},
			IPAddresses: ingressHostIPAddresses(host),
			ValidFor:    365 * 24 * time.Hour, // 1 year
		},
	)
}

func (m *TLSManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	host := strings.ToLower(strings.TrimSpace(hello.ServerName))
	if host == "" {
		return nil, fmt.Errorf("ingress tls SNI is required")
	}
	if err := m.EnsureHost(host); err != nil {
		return nil, err
	}
	certPath, keyPath, err := ingressCertPaths(host)
	if err != nil {
		return nil, err
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

func normalizeTLSHosts(hosts []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, raw := range hosts {
		host := strings.ToLower(strings.TrimSpace(raw))
		if host == "" || seen[host] {
			continue
		}
		seen[host] = true
		out = append(out, host)
	}
	sort.Strings(out)
	return out
}

func validateTLSHost(host string) error {
	if host == "" {
		return fmt.Errorf("ingress tls host is required")
	}
	if strings.Contains(host, "*") {
		return fmt.Errorf("ingress tls wildcard host is not supported yet: %s", host)
	}
	if strings.ContainsAny(host, "/\\") || host == "." || host == ".." || strings.Contains(host, "..") {
		return fmt.Errorf("invalid ingress tls host: %s", host)
	}
	if strings.Contains(host, ":") && net.ParseIP(host) == nil {
		return fmt.Errorf("ingress tls host must not include port: %s", host)
	}
	return nil
}

func ingressHostIPAddresses(host string) []net.IP {
	ip := net.ParseIP(host)
	if ip == nil {
		return nil
	}
	return []net.IP{ip}
}

func ingressCertPaths(host string) (string, string, error) {
	if err := validateTLSHost(host); err != nil {
		return "", "", err
	}
	dirName := strings.ReplaceAll(host, ":", "_")
	dir := filepath.Join(utils.IngressCertDir, "hosts", dirName)
	return filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key"), nil
}
