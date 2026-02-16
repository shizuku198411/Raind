package cert

import (
	"condenser/internal/api/http/logger"
	apimodel "condenser/internal/api/http/utils"
	"condenser/internal/core/cert"
	"condenser/internal/store/csm"
	"condenser/internal/utils"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{
		certHandler: cert.NewCertManager(),
		csmHandler:  csm.NewCsmManager(csm.NewCsmStore(utils.CsmStorePath)),
	}
}

type RequestHandler struct {
	certHandler cert.CertHandler
	csmHandler  csm.CsmHandler
}

func (h *RequestHandler) SignCSRHandler(w http.ResponseWriter, r *http.Request) {
	csrPem, err := ioReadAllLimit(r.Body, 1<<20) // 1 MiB Limit
	if err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "request body: "+err.Error(), nil)
		return
	}

	csr, err := parseCSRFromPEM(csrPem)
	if err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid csr", nil)
		return
	}

	if err := csr.CheckSignature(); err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "csr signture invalid", nil)
		return
	}

	spiffeId, role, id, err := extractAndValidateSPIFFE(csr, "raind")
	if err != nil {
		apimodel.RespondFail(w, http.StatusForbidden, "spiffe policy denied: "+err.Error(), nil)
		return
	}

	switch role {
	case "container":
	default:
		apimodel.RespondFail(w, http.StatusForbidden, "role not allowed", nil)
		return
	}

	ca, err := loadCA(utils.ClientIssuerCACertPath, utils.ClientIssuerCAKeyPath)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "load ca: "+err.Error(), nil)
		return
	}

	// set log: target
	logger.SetTarget(r.Context(), logger.Target{
		CommonName: csr.Subject.CommonName,
		SANURIs:    csr.URIs,
	})

	// issue client serr
	certDer, containerId, newSpiffe, err := h.certHandler.IssueClientCertFromCSR(csr, ca.Cert, ca.Key, spiffeId, id, 365*24*time.Hour)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "issue cert: "+err.Error(), nil)
		return
	}

	// update spiffe on CSM
	if err := h.csmHandler.UpdateSpiffe(containerId, newSpiffe); err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "update spiffe: "+err.Error(), nil)
		return
	}

	w.Header().Set("Content-Type", "application/x-pem-file")
	_ = pem.Encode(w, &pem.Block{Type: "CERTIFICATE", Bytes: certDer})
}

func ioReadAllLimit(r io.Reader, limit int64) ([]byte, error) {
	lr := &io.LimitedReader{R: r, N: limit + 1}
	b, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, errors.New("request too large")
	}
	return b, nil
}

func parseCSRFromPEM(pemBytes []byte) (*x509.CertificateRequest, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("no pem block")
	}
	if block.Type != "CERTIFICATE REQUEST" && block.Type != "NEW CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("unexpected pem type: %s", block.Type)
	}
	return x509.ParseCertificateRequest(block.Bytes)
}

func extractAndValidateSPIFFE(csr *x509.CertificateRequest, trustDomain string) (*url.URL, string, string, error) {
	if len(csr.URIs) != 1 {
		return nil, "", "", errors.New("exactly one URI SAN is required")
	}
	u := csr.URIs[0]
	if u.Scheme != "spiffe" {
		return nil, "", "", errors.New("scheme must be spiffe")
	}
	if u.Host != trustDomain {
		return nil, "", "", fmt.Errorf("trust domain must be %q", trustDomain)
	}

	// path: /<role>/<id>
	path := strings.TrimPrefix(u.Path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		return nil, "", "", errors.New("path must be /<role>/<id>")
	}
	role, id := parts[0], parts[1]
	if role == "" || id == "" {
		return nil, "", "", errors.New("role/id empty")
	}
	if !isSafeID(role) || !isSafeID(id) {
		return nil, "", "", errors.New("invalid characters in role/id")
	}

	return u, role, id, nil
}

func isSafeID(s string) bool {
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			continue
		}
		return false
	}
	return true
}

type CA struct {
	Cert *x509.Certificate
	Key  *rsa.PrivateKey
}

func loadCA(caCertPath, caKeyPath string) (*CA, error) {
	certPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, err
	}
	keyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		return nil, err
	}

	cert, err := parseCertPEM(certPEM)
	if err != nil {
		return nil, err
	}
	key, err := parseRSAPrivateKeyPEM(keyPEM)
	if err != nil {
		return nil, err
	}

	return &CA{Cert: cert, Key: key}, nil
}

func parseCertPEM(pemBytes []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("invalid cert PEM")
	}
	return x509.ParseCertificate(block.Bytes)
}

func parseRSAPrivateKeyPEM(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("invalid key PEM")
	}

	if block.Type == "RSA PRIVATE KEY" {
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	}

	if block.Type == "PRIVATE KEY" {
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key is not RSA")
		}
		return rsaKey, nil
	}

	return nil, errors.New("unsupported key PEM type: " + block.Type)
}
