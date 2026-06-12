package http

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequireSPIFFERejectsMissingTLS(t *testing.T) {
	handler := RequireSPIFFE("spiffe://raind/cli/")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireSPIFFERejectsMissingPeerCertificate(t *testing.T) {
	handler := RequireSPIFFE("spiffe://raind/cli/")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestRequireSPIFFEAllowsMatchingPrefix(t *testing.T) {
	handler := RequireSPIFFE("spiffe://raind/cli/")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.TLS = (&tlsState{uri: "spiffe://raind/cli/admin"}).connectionState()
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRequireSPIFFERejectsWrongPrefix(t *testing.T) {
	handler := RequireSPIFFE("spiffe://raind/cli/")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.TLS = (&tlsState{uri: "spiffe://raind/droplet/container"}).connectionState()
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireSPIFFERejectsEmptyURISANWithoutPanic(t *testing.T) {
	handler := RequireSPIFFE("spiffe://raind/cli/")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.TLS = (&tlsState{}).connectionState()
	rec := httptest.NewRecorder()

	require.NotPanics(t, func() {
		handler.ServeHTTP(rec, req)
	})
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

type tlsState struct {
	uri string
}

func (s *tlsState) connectionState() *tls.ConnectionState {
	cert := &x509.Certificate{}
	if s.uri != "" {
		u, _ := url.Parse(s.uri)
		cert.URIs = []*url.URL{u}
	}
	return &tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}
}
