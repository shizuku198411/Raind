package networkpolicy

import (
	"net/http"
	"time"

	apimodel "raind/internal/condenser/api/http/utils"
	corenetworkpolicy "raind/internal/condenser/core/networkpolicy"
	"raind/internal/condenser/store/netpol"

	"github.com/go-chi/chi/v5"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{service: corenetworkpolicy.NewService()}
}

type RequestHandler struct {
	service *corenetworkpolicy.Service
}

func (h *RequestHandler) GetNetworkPolicyList(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.List(r.URL.Query().Get("namespace"))
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "list failed: "+err.Error(), nil)
		return
	}
	res := make([]NetworkPolicySummary, 0, len(list))
	for _, info := range list {
		res = append(res, toSummary(info))
	}
	apimodel.RespondSuccess(w, http.StatusOK, "networkpolicy list", res)
}

func (h *RequestHandler) GetNetworkPolicy(w http.ResponseWriter, r *http.Request) {
	idOrName := chi.URLParam(r, "idOrName")
	info, err := h.service.Get(idOrName, r.URL.Query().Get("namespace"))
	if err != nil {
		apimodel.RespondFail(w, http.StatusNotFound, "get failed: "+err.Error(), nil)
		return
	}
	apimodel.RespondSuccess(w, http.StatusOK, "networkpolicy detail", toSummary(info))
}

func (h *RequestHandler) RemoveNetworkPolicy(w http.ResponseWriter, r *http.Request) {
	idOrName := chi.URLParam(r, "idOrName")
	info, err := h.service.Remove(idOrName, r.URL.Query().Get("namespace"))
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "remove failed: "+err.Error(), nil)
		return
	}
	apimodel.RespondSuccess(w, http.StatusOK, "networkpolicy removed", toSummary(info))
}

func toSummary(info netpol.NetworkPolicyInfo) NetworkPolicySummary {
	return NetworkPolicySummary{
		NetworkPolicyId: info.NetworkPolicyId,
		Name:            info.Name,
		Namespace:       info.Namespace,
		PodSelector:     info.PodSelector,
		IngressRules:    len(info.Ingress),
		EgressRules:     len(info.Egress),
		GeneratedRules:  len(info.GeneratedRuleIds),
		CreatedAt:       info.CreatedAt.Format(time.RFC3339),
	}
}
