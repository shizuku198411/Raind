package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRespondSuccessWritesEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()

	RespondSuccess(rec, http.StatusCreated, "created", map[string]string{"id": "c1"})

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	var got ApiResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "success", got.Status)
	assert.Equal(t, "created", got.Message)
}

func TestRespondFailWritesEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()

	RespondFail(rec, http.StatusBadRequest, "bad request", nil)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var got ApiResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "fail", got.Status)
	assert.Equal(t, "bad request", got.Message)
}

func TestDecodeRequestBodyRejectsUnknownFields(t *testing.T) {
	var dst struct {
		Name string `json:"name"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"ok","unknown":true}`))

	err := DecodeRequestBody(req, &dst)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown field")
}

func TestDecodeRequestBodyRejectsInvalidJSON(t *testing.T) {
	var dst struct {
		Name string `json:"name"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":`))

	err := DecodeRequestBody(req, &dst)

	require.Error(t, err)
}

func TestDecodeRequestBodyRejectsTooLargeJSON(t *testing.T) {
	var dst struct {
		Name string `json:"name"`
	}
	body := `{"name":"` + strings.Repeat("a", int(MaxJSONBodyBytes)) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))

	err := DecodeRequestBody(req, &dst)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRequestBodyTooLarge)
	assert.Equal(t, http.StatusRequestEntityTooLarge, DecodeErrorStatus(err))
}

func TestDecodeRequestBodyRejectsMultipleJSONValues(t *testing.T) {
	var dst struct {
		Name string `json:"name"`
	}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"ok"}{"name":"extra"}`))

	err := DecodeRequestBody(req, &dst)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple JSON values")
}

func TestReadLimitedBodyRejectsTooLargeManifest(t *testing.T) {
	body := strings.NewReader(strings.Repeat("a", int(MaxManifestBodyBytes)+1))

	_, err := ReadLimitedBody(body, MaxManifestBodyBytes)

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrRequestBodyTooLarge)
	assert.Equal(t, http.StatusRequestEntityTooLarge, DecodeErrorStatus(err))
}

func TestStreamJsonWritesNDJSONEvent(t *testing.T) {
	rec := httptest.NewRecorder()

	StreamJson(rec, StreamEvent{Status: "downloading", ID: "layer1", Current: 1, Total: 2})

	assert.Equal(t, "application/x-ndjson", rec.Header().Get("Content-Type"))
	var got StreamEvent
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "downloading", got.Status)
	assert.Equal(t, "layer1", got.ID)
}
