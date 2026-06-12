package network

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateBridgeInvalidJSONReturnsFailEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()

	NewRequestHandler().CreateBridge(rec, httptest.NewRequest(http.MethodPost, "/v1/networks", strings.NewReader(`{"bridge":`)))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var got struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "fail", got.Status)
}

func TestDeleteBridgeMissingBridgeReturnsFailEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()

	NewRequestHandler().DeleteBridge(rec, httptest.NewRequest(http.MethodDelete, "/v1/networks//actions/delete", nil))

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
