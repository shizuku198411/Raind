package image

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	coreimage "raind/internal/condenser/core/image"
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

func TestPullImageTooLargeJSONReturnsRequestEntityTooLarge(t *testing.T) {
	rec := httptest.NewRecorder()
	body := `{"image":"` + strings.Repeat("a", 1<<20) + `"}`

	NewRequestHandler().PullImage(rec, httptest.NewRequest(http.MethodPost, "/v1/images", strings.NewReader(body)))

	assertFailEnvelope(t, rec, http.StatusRequestEntityTooLarge)
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

func TestBuildImageStreamWritesProgressEvents(t *testing.T) {
	rec := httptest.NewRecorder()
	handler := &RequestHandler{serviceHandler: &fakeBuildImageService{}}

	handler.BuildImage(rec, httptest.NewRequest(http.MethodPost, "/v1/images/build?stream=1&tag=local/test:latest", bytes.NewReader(buildContextTar(t))))

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/x-ndjson", rec.Header().Get("Content-Type"))
	body := rec.Body.String()
	assert.Contains(t, body, `"status":"extracted"`)
	assert.Contains(t, body, `"status":"building"`)
	assert.Contains(t, body, `"status":"running"`)
	assert.Contains(t, body, `"status":"success"`)
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

func buildContextTar(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	content := []byte("FROM scratch\n")
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: "Dockerfile", Mode: 0o644, Size: int64(len(content))}))
	_, err := tw.Write(content)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	return buf.Bytes()
}

type fakeBuildImageService struct{}

func (f *fakeBuildImageService) Pull(coreimage.ServicePullModel) error     { return nil }
func (f *fakeBuildImageService) Remove(coreimage.ServiceRemoveModel) error { return nil }
func (f *fakeBuildImageService) Build(model coreimage.ServiceBuildModel) (string, error) {
	if model.Progress != nil {
		model.Progress(coreimage.PullProgressEvent{Status: "running", ID: "RUN", Detail: "echo ok"})
	}
	return "local/test:latest", nil
}
func (f *fakeBuildImageService) GetImageConfig(string) (coreimage.ImageConfigFile, error) {
	return coreimage.ImageConfigFile{}, nil
}
func (f *fakeBuildImageService) GetImageList() ([]coreimage.ImageInfo, error) {
	return nil, nil
}
func (f *fakeBuildImageService) GetImageStatus(string) (coreimage.ImageStatusInfo, error) {
	return coreimage.ImageStatusInfo{}, errors.New("not implemented")
}
func (f *fakeBuildImageService) GetImageFsInfo(string) (coreimage.ImageFsInfo, error) {
	return coreimage.ImageFsInfo{}, errors.New("not implemented")
}
