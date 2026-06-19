package configmap

import (
	"net/http"
	"time"

	apimodel "raind/internal/condenser/api/http/utils"
	coreconfigmap "raind/internal/condenser/core/configmap"

	"github.com/go-chi/chi/v5"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{service: coreconfigmap.NewConfigMapService()}
}

type RequestHandler struct {
	service *coreconfigmap.ConfigMapService
}

func (h *RequestHandler) GetConfigMapList(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.List(r.URL.Query().Get("namespace"))
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "list failed: "+err.Error(), nil)
		return
	}
	res := make([]ConfigMapSummary, 0, len(list))
	for _, cm := range list {
		res = append(res, ConfigMapSummary{
			ConfigMapId: cm.ConfigMapId,
			Name:        cm.Name,
			Namespace:   cm.Namespace,
			Data:        cm.Data,
			CreatedAt:   cm.CreatedAt.Format(time.RFC3339),
		})
	}
	apimodel.RespondSuccess(w, http.StatusOK, "configmap list", res)
}

func (h *RequestHandler) GetConfigMap(w http.ResponseWriter, r *http.Request) {
	idOrName := chi.URLParam(r, "idOrName")
	info, err := h.service.Get(idOrName, r.URL.Query().Get("namespace"))
	if err != nil {
		apimodel.RespondFail(w, http.StatusNotFound, "get failed: "+err.Error(), nil)
		return
	}
	apimodel.RespondSuccess(w, http.StatusOK, "configmap detail", info)
}

func (h *RequestHandler) RemoveConfigMap(w http.ResponseWriter, r *http.Request) {
	idOrName := chi.URLParam(r, "idOrName")
	info, err := h.service.Remove(idOrName, r.URL.Query().Get("namespace"))
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "remove failed: "+err.Error(), nil)
		return
	}
	apimodel.RespondSuccess(w, http.StatusOK, "configmap removed", info)
}
