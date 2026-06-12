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

func TestRequireSPIFFEAllowsMatchingURIWhenFirstURIIsWrong(t *testing.T) {
	handler := RequireSPIFFE("spiffe://raind/cli/")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.TLS = (&tlsState{uris: []string{"spiffe://raind/droplet/container", "spiffe://raind/cli/admin"}}).connectionState()
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
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

func TestRequireCLIScopeAllowsAdminRole(t *testing.T) {
	handler := RequireCLIIdentity(RequireCLIScope(ScopePolicyWrite)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := CLIIdentityFromContext(r.Context())
		require.True(t, ok)
		assert.Equal(t, "admin", identity.Role)
		w.WriteHeader(http.StatusNoContent)
	})))
	req := httptest.NewRequest(http.MethodPost, "/v1/policies/commit", nil)
	req.TLS = (&tlsState{uri: "spiffe://raind/cli/admin"}).connectionState()
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRequireCLIScopeAllowsExplicitScope(t *testing.T) {
	handler := RequireCLIIdentity(RequireCLIScope(ScopeContainerExec)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	req := httptest.NewRequest(http.MethodPost, "/v1/containers/cid/actions/exec", nil)
	req.TLS = (&tlsState{uri: "spiffe://raind/cli/operator/read,container.exec"}).connectionState()
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestRequireCLIScopeRejectsMissingScope(t *testing.T) {
	handler := RequireCLIIdentity(RequireCLIScope(ScopePolicyWrite)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	req := httptest.NewRequest(http.MethodPost, "/v1/policies/commit", nil)
	req.TLS = (&tlsState{uri: "spiffe://raind/cli/read"}).connectionState()
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireCLIIdentityRejectsNonCLI(t *testing.T) {
	handler := RequireCLIIdentity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.TLS = (&tlsState{uri: "spiffe://raind/droplet/container"}).connectionState()
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

type tlsState struct {
	uri  string
	uris []string
}

func (s *tlsState) connectionState() *tls.ConnectionState {
	cert := &x509.Certificate{}
	if s.uri != "" {
		u, _ := url.Parse(s.uri)
		cert.URIs = []*url.URL{u}
	}
	for _, rawURI := range s.uris {
		u, _ := url.Parse(rawURI)
		cert.URIs = append(cert.URIs, u)
	}
	return &tls.ConnectionState{PeerCertificates: []*x509.Certificate{cert}}
}
