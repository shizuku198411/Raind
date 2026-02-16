package cert

import (
	"crypto/rsa"
	"crypto/x509"
	"net/url"
	"time"
)

type CertHandler interface {
	EnsureSelfSignedCert(certPath string, keyPath string, cfg CertConfig) error
	EnsureClientCACert(certPath string, keyPath string, cfg CertConfig) error
	IssueClientCert(certPath string, keyPath string, CACertPath string, CAKeyPath string, cfg ClientCertConfig) error
	IssueClientCertFromCSR(csr *x509.CertificateRequest, caCert *x509.Certificate, caKey *rsa.PrivateKey, spiffe *url.URL, id string, validFor time.Duration) ([]byte, string, string, error)
}
