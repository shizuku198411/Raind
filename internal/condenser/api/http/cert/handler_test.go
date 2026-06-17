package cert

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"raind/internal/condenser/store/csm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractAuthenticatedDropletID(t *testing.T) {
	req := newRequestWithPeerSPIFFE(t, "spiffe://raind/droplet/container")

	got, err := extractAuthenticatedDropletID(req)

	require.NoError(t, err)
	assert.Equal(t, "container", got)
}

func TestExtractAuthenticatedDropletIDRejectsMissingTLS(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/pki/sign", nil)

	_, err := extractAuthenticatedDropletID(req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing peer certificate")
}

func TestExtractAuthenticatedDropletIDRejectsContainerSPIFFE(t *testing.T) {
	req := newRequestWithPeerSPIFFE(t, "spiffe://raind/container/cid-1")

	_, err := extractAuthenticatedDropletID(req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing droplet SPIFFE ID")
}

func TestExtractAuthenticatedDropletIDRejectsDifferentTrustDomain(t *testing.T) {
	req := newRequestWithPeerSPIFFE(t, "spiffe://other/droplet/container")

	_, err := extractAuthenticatedDropletID(req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing droplet SPIFFE ID")
}

func TestAuthorizeDropletForContainerAllowsAssignedCreatingContainer(t *testing.T) {
	h := newTestRequestHandler(t, csm.StoreContainerRequest{
		ContainerId:   "cid-1",
		State:         "creating",
		ContainerName: "web",
		DropletId:     "container",
	})
	req := newRequestWithPeerSPIFFE(t, "spiffe://raind/droplet/container")

	err := h.authorizeDropletForContainer(req, "cid-1")

	require.NoError(t, err)
}

func TestAuthorizeDropletForContainerRejectsDifferentDroplet(t *testing.T) {
	h := newTestRequestHandler(t, csm.StoreContainerRequest{
		ContainerId:   "cid-1",
		State:         "creating",
		ContainerName: "web",
		DropletId:     "other",
	})
	req := newRequestWithPeerSPIFFE(t, "spiffe://raind/droplet/container")

	err := h.authorizeDropletForContainer(req, "cid-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "different droplet")
}

func TestAuthorizeDropletForContainerRejectsMissingAssignment(t *testing.T) {
	h := newTestRequestHandler(t, csm.StoreContainerRequest{
		ContainerId:   "cid-1",
		State:         "creating",
		ContainerName: "web",
	})
	req := newRequestWithPeerSPIFFE(t, "spiffe://raind/droplet/container")

	err := h.authorizeDropletForContainer(req, "cid-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no assigned droplet")
}

func TestAuthorizeDropletForContainerRejectsNonCreatingContainer(t *testing.T) {
	h := newTestRequestHandler(t, csm.StoreContainerRequest{
		ContainerId:   "cid-1",
		State:         "running",
		ContainerName: "web",
		DropletId:     "container",
	})
	req := newRequestWithPeerSPIFFE(t, "spiffe://raind/droplet/container")

	err := h.authorizeDropletForContainer(req, "cid-1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not creating")
}

func newRequestWithPeerSPIFFE(t *testing.T, rawURI string) *http.Request {
	t.Helper()
	u, err := url.Parse(rawURI)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/v1/pki/sign", nil)
	req.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{{URIs: []*url.URL{u}}}}
	return req
}

func newTestRequestHandler(t *testing.T, req csm.StoreContainerRequest) *RequestHandler {
	t.Helper()
	manager := csm.NewCsmManager(csm.NewCsmStore(filepath.Join(t.TempDir(), "csm.json")))
	require.NoError(t, manager.StoreContainer(req))
	return &RequestHandler{csmHandler: manager}
}
