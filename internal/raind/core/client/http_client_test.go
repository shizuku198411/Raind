package httpclient

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHttpClientNewRequestBuildsURLAndContentType(t *testing.T) {
	client := &HttpClient{BaseUrl: "https://example.test"}

	err := client.NewRequest(http.MethodPost, "/v1/images", []byte(`{"image":"alpine"}`))

	require.NoError(t, err)
	assert.Equal(t, "https://example.test/v1/images", client.Request.URL.String())
	assert.Equal(t, "application/json", client.Request.Header.Get("Content-Type"))
}

func TestHttpClientIsStatusOk(t *testing.T) {
	client := &HttpClient{}

	assert.True(t, client.IsStatusOk(&http.Response{StatusCode: http.StatusOK}))
	assert.True(t, client.IsStatusOk(&http.Response{StatusCode: http.StatusCreated}))
	assert.True(t, client.IsStatusOk(&http.Response{StatusCode: http.StatusAccepted}))
	assert.False(t, client.IsStatusOk(&http.Response{StatusCode: http.StatusBadRequest}))
}

func TestReadStreamEventsReturnsLastSuccess(t *testing.T) {
	body := strings.NewReader(`{"status":"downloading","id":"layer","current":1,"total":2}
{"status":"success","id":"container-1","data":{"id":"container-1"}}
`)

	event, err := ReadStreamEvents(body)

	require.NoError(t, err)
	assert.Equal(t, "success", event.Status)
	assert.Equal(t, "container-1", event.ID)
}

func TestReadStreamEventsReturnsErrorEvent(t *testing.T) {
	body := strings.NewReader(`{"status":"error","error":"pull failed"}`)

	event, err := ReadStreamEvents(body)

	require.Error(t, err)
	assert.Equal(t, "error", event.Status)
	assert.Contains(t, err.Error(), "pull failed")
}
