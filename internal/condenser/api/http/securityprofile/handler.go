package securityprofile

import (
	"net/http"
	apimodel "raind/internal/condenser/api/http/utils"
	core "raind/internal/condenser/core/securityprofile"

	"github.com/go-chi/chi/v5"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{service: core.NewService()}
}

type RequestHandler struct {
	service *core.Service
}

// ListSecurityProfiles godoc
// @Summary list security profiles
// @Description list built-in and registered security profiles
// @Tags security
// @Produce json
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/security/profiles [get]
func (h *RequestHandler) ListSecurityProfiles(w http.ResponseWriter, r *http.Request) {
	apimodel.RespondSuccess(w, http.StatusOK, "security profile list", ListSecurityProfileResponse{Profiles: h.service.List()})
}

// ShowSecurityProfile godoc
// @Summary show security profile
// @Description show security profile detail
// @Tags security
// @Produce json
// @Param name path string true "Security profile name"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/security/profiles/{name} [get]
func (h *RequestHandler) ShowSecurityProfile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing security profile name", ShowSecurityProfileResponse{})
		return
	}

	profile, err := h.service.Get(name)
	if err != nil {
		apimodel.RespondFail(w, http.StatusNotFound, err.Error(), ShowSecurityProfileResponse{})
		return
	}

	apimodel.RespondSuccess(w, http.StatusOK, "security profile", ShowSecurityProfileResponse{Profile: profile})
}

// RegisterSecurityProfile godoc
// @Summary register security profile
// @Description register custom security profile
// @Tags security
// @Accept json
// @Produce json
// @Success 201 {object} apimodel.ApiResponse
// @Router /v1/security/profiles [post]
func (h *RequestHandler) RegisterSecurityProfile(w http.ResponseWriter, r *http.Request) {
	var req RegisterSecurityProfileRequest
	if err := apimodel.DecodeRequestBody(r, &req); err != nil {
		apimodel.RespondFail(w, apimodel.DecodeErrorStatus(err), "invalid security profile request: "+err.Error(), RegisterSecurityProfileResponse{})
		return
	}
	profile, err := h.service.Register(req)
	if err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, err.Error(), RegisterSecurityProfileResponse{})
		return
	}
	apimodel.RespondSuccess(w, http.StatusCreated, "security profile registered", RegisterSecurityProfileResponse{Profile: profile})
}

// DeleteSecurityProfile godoc
// @Summary delete security profile
// @Description delete custom security profile
// @Tags security
// @Produce json
// @Param name path string true "Security profile name"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/security/profiles/{name} [delete]
func (h *RequestHandler) DeleteSecurityProfile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing security profile name", DeleteSecurityProfileResponse{})
		return
	}
	if err := h.service.Delete(name); err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, err.Error(), DeleteSecurityProfileResponse{})
		return
	}
	apimodel.RespondSuccess(w, http.StatusOK, "security profile deleted", DeleteSecurityProfileResponse{Name: name})
}
