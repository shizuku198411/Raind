package policy

import (
	"condenser/internal/core/policy"
	"net/http"

	"condenser/internal/api/http/logger"
	apimodel "condenser/internal/api/http/utils"

	"github.com/go-chi/chi/v5"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{
		policyServiceHandler: policy.NewwServicePolicy(),
	}
}

type RequestHandler struct {
	policyServiceHandler policy.PolicyServiceHandler
}

// AddPolicy godoc
// @Summary Add policy
// @Description Add new policy
// @Tags Policy
// @Accept json
// @Produce json
// @Param request body AddPolicyRequest true "policy parameter"
// @Success 201 {object} apimodel.ApiResponse
// @Router /v1/policies [post]
func (h *RequestHandler) AddPolicy(w http.ResponseWriter, r *http.Request) {
	// decode request
	var req AddPolicyRequest
	if err := apimodel.DecodeRequestBody(r, &req); err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), req)
		return
	}

	// set log: target
	logger.SetTarget(r.Context(), logger.Target{
		ChainName:   req.ChainName,
		Source:      req.Source,
		Destination: req.Destination,
		Protocol:    req.Protocol,
		DestPort:    req.DestPort,
		Comment:     req.Comment,
	})

	// service: add policy
	policyId, err := h.policyServiceHandler.AddUserPolicy(
		policy.ServiceAddPolicyModel{
			ChainName:   req.ChainName,
			Source:      req.Source,
			Destination: req.Destination,
			Protocol:    req.Protocol,
			DestPort:    req.DestPort,
			Comment:     req.Comment,
		},
	)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "service failed: "+err.Error(), req)
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "policy created", AddPolicyResponse{Id: policyId})
}

// RemovePolicy godoc
// @Summary remove a policy
// @Description remove an existing policy
// @Tags Policy
// @Param policyId path string true "Policy ID"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/policies/{policyId} [delete]
func (h *RequestHandler) RemovePolicy(w http.ResponseWriter, r *http.Request) {
	policyId := chi.URLParam(r, "policyId")
	if policyId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing policy Id", nil)
		return
	}

	// set log: target
	logger.SetTarget(r.Context(), logger.Target{
		PolicyId: policyId,
	})

	// service: remove policy
	err := h.policyServiceHandler.RemoveUserPolicy(
		policy.ServiceRemovePolicyModel{
			Id: policyId,
		},
	)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "service failed: "+err.Error(), nil)
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "request accept. remove next commit", nil)
}

// CommitPolicy godoc
// @Summary commit policy
// @Description commit all policy
// @Tags Policy
// @Accept json
// @Produce json
// @Success 201 {object} apimodel.ApiResponse
// @Router /v1/policies/commit [post]
func (h *RequestHandler) CommitPolicy(w http.ResponseWriter, r *http.Request) {
	// service: commit
	err := h.policyServiceHandler.CommitPolicy()
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "service failed: "+err.Error(), nil)
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "policy committed", nil)
}

// RevertPolicy godoc
// @Summary revert policy
// @Description revert policy
// @Tags Policy
// @Accept json
// @Produce json
// @Success 201 {object} apimodel.ApiResponse
// @Router /v1/policies/revert [post]
func (h *RequestHandler) RevertPolicy(w http.ResponseWriter, r *http.Request) {
	// service: revert
	err := h.policyServiceHandler.RevertPolicy()
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "service failed: "+err.Error(), nil)
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "policy reverted", nil)
}

// GetPolicyList godoc
// @Summary get policy list
// @Description get policy
// @Tags Policy
// @Accept json
// @Produce json
// @Param chainName path string true "Chain Name"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/policies [get]
func (h *RequestHandler) GetPolicyList(w http.ResponseWriter, r *http.Request) {
	chainName := chi.URLParam(r, "chain")
	if chainName == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing chain name", nil)
		return
	}

	// service: get list
	policyList := h.policyServiceHandler.GetPolicyList(
		policy.ServiceListModel{
			Chain: chainName,
		},
	)

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "get list success", policyList)
}

// ChangeNSMode godoc
// @Summary change north-south mode
// @Description change north-south mode
// @Tags Policy
// @Accept json
// @Produce json
// @Param request body ChangeNSModeRequest true "mode"
// @Success 201 {object} apimodel.ApiResponse
// @Router /v1/policies/ns/mode [post]
func (h *RequestHandler) ChangeNSMode(w http.ResponseWriter, r *http.Request) {
	// decode request
	var req ChangeNSModeRequest
	if err := apimodel.DecodeRequestBody(r, &req); err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), req)
		return
	}

	// service: change ns mode
	err := h.policyServiceHandler.ChangeNSMode(req.Mode)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "service failed: "+err.Error(), req)
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "North-South Mode changed", req)
}
