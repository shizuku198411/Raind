package http

import (
	"condenser/internal/core/hook"
	"condenser/internal/store/csm"
	"condenser/internal/utils"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"condenser/internal/api/http/logger"
	apimodel "condenser/internal/api/http/utils"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{
		hookServiceHandler: hook.NewHookService(),
		csmHandler:         csm.NewCsmManager(csm.NewCsmStore(utils.CsmStorePath)),
	}
}

type RequestHandler struct {
	hookServiceHandler hook.HookServiceHandler
	csmHandler         csm.CsmHandler
}

// ApplyHook godoc
// @Summary apply hook
// @Description apply hook from droplet
// @Tags hooks
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/hooks/droplet [post]
func (h *RequestHandler) ApplyHook(w http.ResponseWriter, r *http.Request) {
	eventType := r.Header.Get("X-Hook-Event")
	if eventType == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid request", nil)
		return
	}

	// set log: action
	switch eventType {
	case "createRuntime":
		logger.SetAction(r.Context(), "hook.createRuntime")
	case "createContainer":
		logger.SetAction(r.Context(), "hook.createContainer")
	case "poststart":
		logger.SetAction(r.Context(), "hook.poststart")
	case "stopContainer":
		logger.SetAction(r.Context(), "hook.stopContainer")
	case "poststop":
		logger.SetAction(r.Context(), "hook.poststop")
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "read hook body failed: "+err.Error(), nil)
		return
	}
	var st hook.ServiceStateModel
	if err := json.Unmarshal(body, &st); err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), nil)
		return
	}

	// set log: target
	logger.SetTarget(r.Context(), logger.Target{
		ContainerId: st.Id,
	})

	if ok, err := h.validateSpiffe(r, st); !ok {
		apimodel.RespondFail(w, http.StatusForbidden, "validate failed: "+err.Error(), nil)
		return
	}

	// service: hook
	if err := h.hookServiceHandler.HookAction(st, eventType); err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "service hook failed: "+err.Error(), nil)
		return
	}

	apimodel.RespondSuccess(w, http.StatusOK, "hook applied", nil)
}

func (h *RequestHandler) validateSpiffe(r *http.Request, state hook.ServiceStateModel) (bool, error) {
	cert := r.TLS.PeerCertificates[0]
	for _, uri := range cert.URIs {
		u, err := url.Parse(uri.String())
		if err != nil {
			return false, fmt.Errorf("invalid format: %s", uri)
		}

		// validate scheme
		if u.Scheme != "spiffe" {
			return false, fmt.Errorf("invalid scheme: %s", u.Scheme)
		}
		// validate domain
		if u.Host != "raind" {
			return false, fmt.Errorf("invalid domain: %s", u.Host)
		}

		// retrieve container id
		path := strings.TrimPrefix(u.Path, "/")
		parts := strings.Split(path, "/")
		if len(parts) != 2 || parts[0] != "container" {
			return false, fmt.Errorf("invalid spiffe path: %s", path)
		}
		containerId := parts[1]
		if containerId == "" {
			return false, errors.New("container id empty")
		}

		// validate container id
		// check if the spiffe's id exist
		if ok := h.csmHandler.IsContainerExist(containerId); !ok {
			return false, fmt.Errorf("container: %s not found", containerId)
		}
		// check if the spiffe's id and state's id is same
		if containerId != state.Id {
			return false, fmt.Errorf("SPIFFE ID did not match the state ID: spiffe=%s, state=%s", containerId, state.Id)
		}
	}
	return true, nil
}
