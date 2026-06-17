package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apimodel "raind/internal/condenser/api/http/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateServiceInvalidYAMLReturnsFailEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()

	NewRequestHandler().CreateService(rec, httptest.NewRequest(http.MethodPost, "/v1/services", strings.NewReader("kind: [")))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var got struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "fail", got.Status)
}

func TestCreateServiceRejectsTooLargeBody(t *testing.T) {
	rec := httptest.NewRecorder()
	body := strings.NewReader(strings.Repeat("a", int(apimodel.MaxManifestBodyBytes)+1))

	NewRequestHandler().CreateService(rec, httptest.NewRequest(http.MethodPost, "/v1/services", body))

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	var got struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "fail", got.Status)
}
