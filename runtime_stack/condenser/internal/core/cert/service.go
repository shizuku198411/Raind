package cert

import (
	"condenser/internal/store/csm"
	"condenser/internal/utils"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"strings"
	"time"
)

func NewCertManager() *CertManager {
	return &CertManager{
		csmHandler: csm.NewCsmManager(csm.NewCsmStore(utils.CsmStorePath)),
	}
}

type CertManager struct {
	csmHandler csm.CsmHandler
}

func (m *CertManager) EnsureSelfSignedCert(certPath string, keyPath string, cfg CertConfig) error {
	if m.isFileExists(certPath) && m.isFileExists(keyPath) {
		return nil
	}

	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: cfg.CommonName,
		},
		NotBefore: time.Now().Add(-1 * time.Hour),
		NotAfter:  time.Now().Add(cfg.ValidFor),
		KeyUsage: x509.KeyUsageDigitalSignature |
			x509.KeyUsageKeyEncipherment |
			x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              cfg.DNSNames,
		IPAddresses:           cfg.IPAddresses,
	}

	derBytes, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&privKey.PublicKey,
		privKey,
	)
	if err != nil {
		return err
	}
	if err := m.writePem(certPath, "CERTIFICATE", derBytes, 0644); err != nil {
		return err
	}

	if err := m.writePem(keyPath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(privKey), 0600); err != nil {
		return err
	}

	return nil
}

func (m *CertManager) EnsureClientCACert(certPath string, keyPath string, cfg CertConfig) error {
	if m.isFileExists(certPath) && m.isFileExists(keyPath) {
		return nil
	}

	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return err
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: cfg.CommonName,
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(cfg.ValidFor),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           nil,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&privKey.PublicKey,
		privKey,
	)
	if err != nil {
		return err
	}
	if err := m.writePem(certPath, "CERTIFICATE", derBytes, 0644); err != nil {
		return err
	}

	if err := m.writePem(keyPath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(privKey), 0600); err != nil {
		return err
	}

	return nil
}

func (m *CertManager) IssueClientCert(certPath string, keyPath string, CACertPath string, CAKeyPath string, cfg ClientCertConfig) error {
	if m.isFileExists(certPath) && m.isFileExists(keyPath) {
		return nil
	}

	privKey, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return err
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}

	uri, err := url.Parse(cfg.SpiiffeId)
	if err != nil || uri.Scheme != "spiffe" || uri.Host == "" {
		return fmt.Errorf("invalid SPIFFE ID: %q", cfg.SpiiffeId)
	}

	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: cfg.CommonName,
		},
		NotBefore: time.Now().Add(-5 * time.Minute),
		NotAfter:  time.Now().Add(cfg.ValidFor),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
		},
		BasicConstraintsValid: true,
		IsCA:                  false,
		URIs:                  []*url.URL{uri},
	}

	// load CA cert/key
	caCert, caKey, err := m.loadCertAndKey(CACertPath, CAKeyPath)
	if err != nil {
		return err
	}

	derBytes, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		caCert,
		&privKey.PublicKey,
		caKey,
	)
	if err != nil {
		return err
	}
	if err := m.writePem(certPath, "CERTIFICATE", derBytes, 0644); err != nil {
		return err
	}

	if err := m.writePem(keyPath, "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(privKey), 0600); err != nil {
		return err
	}

	return nil
}

func (m *CertManager) loadCertAndKey(certPath string, keyPath string) (*x509.Certificate, *rsa.PrivateKey, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, err
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, err
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		return nil, nil, errors.New("invalid cert PEM")
	}
	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return nil, nil, errors.New("invalid key PEM")
	}
	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, err
	}

	return cert, key, nil
}

func (m *CertManager) writePem(path string, typ string, der []byte, perm os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	defer f.Close()

	return pem.Encode(f, &pem.Block{
		Type:  typ,
		Bytes: der,
	})
}

func (m *CertManager) isFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (m *CertManager) IssueClientCertFromCSR(
	csr *x509.CertificateRequest, caCert *x509.Certificate, caKey *rsa.PrivateKey,
	spiffe *url.URL, id string, validFor time.Duration,
) ([]byte, string, string, error) {
	// validate container id
	path := strings.TrimPrefix(spiffe.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || parts[0] != "container" {
		return nil, "", "", fmt.Errorf("invalid SPIFFE ID format")
	}
	containerId := parts[1]
	if containerId == "" {
		return nil, "", "", fmt.Errorf("container id empty")
	}
	// check if the container id exxist
	if ok := m.csmHandler.IsContainerExist(containerId); !ok {
		return nil, "", "", fmt.Errorf("invalid container id")
	}
	// check the current contianer status is creating
	containerInfo, _ := m.csmHandler.GetContainerById(containerId)
	if containerInfo.State != "creating" {
		return nil, "", "", fmt.Errorf("invalid container id")
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, "", "", err
	}

	// re-build SPIFFE ID
	newSpiffeId := url.URL{
		Scheme: "spiffe",
		Host:   "raind",
		Path:   "container/" + containerInfo.ContainerId,
	}

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: "raind-client",
		},
		NotBefore:             time.Now().Add(-5 * time.Minute),
		NotAfter:              time.Now().Add(validFor),
		BasicConstraintsValid: true,
		IsCA:                  false,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageClientAuth,
		},
		URIs: []*url.URL{&newSpiffeId},
	}

	certDer, err := x509.CreateCertificate(rand.Reader, template, caCert, csr.PublicKey, caKey)
	return certDer, containerId, newSpiffeId.String(), err
}

func LoadCertPoolFromFile(path string) (*x509.CertPool, error) {
	pemBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(pemBytes); !ok {
		return nil, errors.New("failed to append CA cert")
	}
	return pool, nil
}
