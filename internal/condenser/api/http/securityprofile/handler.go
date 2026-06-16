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
