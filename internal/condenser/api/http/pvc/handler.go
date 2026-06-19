package pvc

import (
	"net/http"
	"time"

	apimodel "raind/internal/condenser/api/http/utils"
	corepvc "raind/internal/condenser/core/pvc"
	"raind/internal/condenser/store/vsm"

	"github.com/go-chi/chi/v5"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{service: corepvc.NewService()}
}

type RequestHandler struct {
	service *corepvc.Service
}

func (h *RequestHandler) GetPVCList(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.List(r.URL.Query().Get("namespace"))
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "list failed: "+err.Error(), nil)
		return
	}
	res := make([]PVCSummary, 0, len(list))
	for _, info := range list {
		res = append(res, toSummary(info))
	}
	apimodel.RespondSuccess(w, http.StatusOK, "persistentvolumeclaim list", res)
}

func (h *RequestHandler) GetPVC(w http.ResponseWriter, r *http.Request) {
	idOrName := chi.URLParam(r, "idOrName")
	info, err := h.service.Get(idOrName, r.URL.Query().Get("namespace"))
	if err != nil {
		apimodel.RespondFail(w, http.StatusNotFound, "get failed: "+err.Error(), nil)
		return
	}
	apimodel.RespondSuccess(w, http.StatusOK, "persistentvolumeclaim detail", toSummary(info))
}

func (h *RequestHandler) RemovePVC(w http.ResponseWriter, r *http.Request) {
	idOrName := chi.URLParam(r, "idOrName")
	info, err := h.service.Remove(idOrName, r.URL.Query().Get("namespace"))
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "remove failed: "+err.Error(), nil)
		return
	}
	apimodel.RespondSuccess(w, http.StatusOK, "persistentvolumeclaim removed", toSummary(info))
}

func toSummary(info vsm.PersistentVolumeClaimInfo) PVCSummary {
	return PVCSummary{
		PVCId:            info.PVCId,
		Name:             info.Name,
		Namespace:        info.Namespace,
		Phase:            info.Phase,
		AccessModes:      info.AccessModes,
		RequestedStorage: info.RequestedStorage,
		RequestedBytes:   info.RequestedBytes,
		ReclaimPolicy:    info.ReclaimPolicy,
		DataPath:         info.DataPath,
		CreatedAt:        info.CreatedAt.Format(time.RFC3339),
	}
}
