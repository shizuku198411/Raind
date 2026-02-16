package image

import (
	"condenser/internal/core/image"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	apimodel "condenser/internal/api/http/utils"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{
		serviceHandler: image.NewImageService(),
	}
}

type RequestHandler struct {
	serviceHandler image.ImageServiceHandler
}

// PullImage godoc
// @Summary pull image
// @Description pull image from registry
// @Tags image
// @Accept json
// @Produce json
// @Param request body PullImageRequest true "Target Image"
// @Success 201 {object} apimodel.ApiResponse
// @Router /v1/images [post]
func (h *RequestHandler) PullImage(w http.ResponseWriter, r *http.Request) {
	// decode request
	var req PullImageRequest
	if err := apimodel.DecodeRequestBody(r, &req); err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), nil)
	}

	// service
	if err := h.serviceHandler.Pull(
		image.ServicePullModel{
			Image: req.Image,
			Os:    req.Os,
			Arch:  req.Arch,
		},
	); err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "pull failed: "+err.Error(), nil)
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "pull completed", req)
}

// RemoveImage godoc
// @Summary remove image
// @Description remove image from local
// @Tags image
// @Accept json
// @Produce json
// @Param request body RemoveImageRequest true "Target Image"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/images [delete]
func (h *RequestHandler) RemoveImage(w http.ResponseWriter, r *http.Request) {
	// decode request
	var req RemoveImageRequest
	if err := apimodel.DecodeRequestBody(r, &req); err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), nil)
	}

	// service
	if err := h.serviceHandler.Remove(
		image.ServiceRemoveModel{
			Image: req.Image,
		},
	); err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "remove failed: "+err.Error(), nil)
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "remove completed", req)
}

// BuildImage godoc
// @Summary build image
// @Description build image from Dripfile-like archive (tar)
// @Tags image
// @Accept application/x-tar
// @Produce json
// @Param tag query string true "Target image tag (e.g. myapp:latest)"
// @Param dripfile query string false "Dripfile path in context (default: Dripfile)"
// @Param network query string false "Bridge interface (default: raind0)"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/images/build [post]
func (h *RequestHandler) BuildImage(w http.ResponseWriter, r *http.Request) {
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing tag query", nil)
		return
	}
	dripfile := r.URL.Query().Get("dripfile")
	if dripfile == "" {
		dripfile = r.URL.Query().Get("dockerfile")
	}
	if dripfile == "" {
		dripfile = "Dripfile"
	}
	network := r.URL.Query().Get("network")

	tmpDir, err := os.MkdirTemp("", "raind-build-context-")
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "create temp dir failed: "+err.Error(), nil)
		return
	}
	defer os.RemoveAll(tmpDir)

	if err := image.ExtractTarToDir(r.Body, tmpDir); err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid build context: "+err.Error(), nil)
		return
	}

	dfPath := filepath.Join(tmpDir, filepath.Clean(dripfile))
	rel, err := filepath.Rel(tmpDir, dfPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid dripfile path", nil)
		return
	}

	result, err := h.serviceHandler.Build(image.ServiceBuildModel{
		Image:        tag,
		ContextDir:   tmpDir,
		DripfilePath: dfPath,
		Network:      network,
	})
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "build failed: "+err.Error(), nil)
		return
	}

	apimodel.RespondSuccess(w, http.StatusOK, "build completed", BuildImageResponse{Image: result})
}

// GetImageList godoc
// @Summary get image list
// @Description get image list in local storage
// @Tags image
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/images [get]
func (h *RequestHandler) GetImageList(w http.ResponseWriter, r *http.Request) {
	// service
	imageList, err := h.serviceHandler.GetImageList()
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "retrieve image list success", imageList)
}

// GetImageStatus godoc
// @Summary get image status
// @Description get image status details
// @Tags image
// @Produce json
// @Param image query string true "Target Image"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/images/status [get]
func (h *RequestHandler) GetImageStatus(w http.ResponseWriter, r *http.Request) {
	imageStr := r.URL.Query().Get("image")
	if imageStr == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing image query", nil)
		return
	}

	status, err := h.serviceHandler.GetImageStatus(imageStr)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "retrieve image status failed: "+err.Error(), nil)
		return
	}

	apimodel.RespondSuccess(w, http.StatusOK, "retrieve image status success", ImageStatusResponse{
		Repository:  status.Repository,
		Reference:   status.Reference,
		Id:          status.Id,
		RepoTags:    status.RepoTags,
		RepoDigests: status.RepoDigests,
		SizeBytes:   status.SizeBytes,
		CreatedAt:   status.CreatedAt,
		User:        status.User,
	})
}

// GetImageFsInfo godoc
// @Summary get image fs info
// @Description get image filesystem usage
// @Tags image
// @Produce json
// @Param image query string true "Target Image"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/images/fs [get]
func (h *RequestHandler) GetImageFsInfo(w http.ResponseWriter, r *http.Request) {
	imageStr := r.URL.Query().Get("image")
	if imageStr == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing image query", nil)
		return
	}

	info, err := h.serviceHandler.GetImageFsInfo(imageStr)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "retrieve image fs info failed: "+err.Error(), nil)
		return
	}

	apimodel.RespondSuccess(w, http.StatusOK, "retrieve image fs info success", ImageFsInfoResponse{
		Image:     info.Image,
		UsedBytes: info.UsedBytes,
	})
}
