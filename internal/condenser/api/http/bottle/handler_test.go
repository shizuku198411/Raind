package bottle

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

func TestRegisterBottleInvalidYAMLReturnsFailEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()

	NewRequestHandler().RegisterBottle(rec, httptest.NewRequest(http.MethodPost, "/v1/bottle", strings.NewReader("bottle: [")))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var got struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "fail", got.Status)
}

func TestRegisterBottleRejectsTooLargeBody(t *testing.T) {
	rec := httptest.NewRecorder()
	body := strings.NewReader(strings.Repeat("a", int(apimodel.MaxManifestBodyBytes)+1))

	NewRequestHandler().RegisterBottle(rec, httptest.NewRequest(http.MethodPost, "/v1/bottle", body))

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	var got struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "fail", got.Status)
}
