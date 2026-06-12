package policy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddPolicyInvalidJSONReturnsFailEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()

	NewRequestHandler().AddPolicy(rec, httptest.NewRequest(http.MethodPost, "/v1/policies", strings.NewReader(`{"chain":`)))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var got struct {
		Status string `json:"status"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "fail", got.Status)
}

func TestChangeNSModeInvalidJSONReturnsFailEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()

	NewRequestHandler().ChangeNSMode(rec, httptest.NewRequest(http.MethodPost, "/v1/policies/ns/mode", strings.NewReader(`{"mode":`)))

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
