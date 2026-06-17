package websocket

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckOriginAllowsRequestsWithoutOrigin(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "https://127.0.0.1:7755/v1/containers/c/attach", nil)

	assert.True(t, h.checkOrigin(req))
}

func TestCheckOriginAllowsSameOriginRequests(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "https://127.0.0.1:7755/v1/containers/c/attach", nil)
	req.Header.Set("Origin", "https://127.0.0.1:7755")

	assert.True(t, h.checkOrigin(req))
}

func TestCheckOriginRejectsCrossOriginRequestsByDefault(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "https://127.0.0.1:7755/v1/containers/c/attach", nil)
	req.Header.Set("Origin", "https://example.invalid")

	assert.False(t, h.checkOrigin(req))
}

func TestCheckOriginRejectsMalformedOrigin(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "https://127.0.0.1:7755/v1/containers/c/attach", nil)
	req.Header.Set("Origin", "://bad-origin")

	assert.False(t, h.checkOrigin(req))
}

func TestCheckOriginAllowsConfiguredOrigin(t *testing.T) {
	h := &Handler{AllowedOrigins: []string{"https://ui.example.test"}}
	req := httptest.NewRequest(http.MethodGet, "https://127.0.0.1:7755/v1/containers/c/attach", nil)
	req.Header.Set("Origin", "https://ui.example.test")

	assert.True(t, h.checkOrigin(req))
}

func TestCheckOriginRejectsUnconfiguredOrigin(t *testing.T) {
	h := &Handler{AllowedOrigins: []string{"https://ui.example.test"}}
	req := httptest.NewRequest(http.MethodGet, "https://127.0.0.1:7755/v1/containers/c/attach", nil)
	req.Header.Set("Origin", "https://example.invalid")

	assert.False(t, h.checkOrigin(req))
}
