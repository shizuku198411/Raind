package pod

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

func TestApplyPodYamlUnsupportedKindReturnsFailEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()

	NewRequestHandler().ApplyPodYaml(rec, httptest.NewRequest(http.MethodPost, "/v1/resource/apply", strings.NewReader("kind: Widget\nmetadata:\n  name: x\n")))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var got struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "fail", got.Status)
}

func TestApplyPodYamlRejectsTooLargeBody(t *testing.T) {
	rec := httptest.NewRecorder()
	body := strings.NewReader(strings.Repeat("a", int(apimodel.MaxManifestBodyBytes)+1))

	NewRequestHandler().ApplyPodYaml(rec, httptest.NewRequest(http.MethodPost, "/v1/resource/apply", body))

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	var got struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "fail", got.Status)
}

func TestDeleteResourceYamlRejectsTooLargeBody(t *testing.T) {
	rec := httptest.NewRecorder()
	body := strings.NewReader(strings.Repeat("a", int(apimodel.MaxManifestBodyBytes)+1))

	NewRequestHandler().DeleteResourceYaml(rec, httptest.NewRequest(http.MethodPost, "/v1/resource/delete", body))

	require.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	var got struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "fail", got.Status)
}
