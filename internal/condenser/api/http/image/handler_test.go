package image

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullImageInvalidJSONReturnsFailEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()

	NewRequestHandler().PullImage(rec, httptest.NewRequest(http.MethodPost, "/v1/images", strings.NewReader(`{"image":`)))

	assertFailEnvelope(t, rec, http.StatusBadRequest)
}

func TestRemoveImageMissingImageReturnsFailEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()

	NewRequestHandler().RemoveImage(rec, httptest.NewRequest(http.MethodDelete, "/v1/images", strings.NewReader(`{}`)))

	assertFailEnvelope(t, rec, http.StatusBadRequest)
}

func TestImageStatusMissingQueryReturnsFailEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()

	NewRequestHandler().GetImageStatus(rec, httptest.NewRequest(http.MethodGet, "/v1/images/status", nil))

	assertFailEnvelope(t, rec, http.StatusBadRequest)
}

func assertFailEnvelope(t *testing.T, rec *httptest.ResponseRecorder, code int) {
	t.Helper()
	require.Equal(t, code, rec.Code)
	var got struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "fail", got.Status)
	assert.NotEmpty(t, got.Message)
}
