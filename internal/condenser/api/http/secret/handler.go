package secret

import (
	"net/http"
	"sort"
	"time"

	apimodel "raind/internal/condenser/api/http/utils"
	coresecret "raind/internal/condenser/core/secret"
	"raind/internal/condenser/store/sec"

	"github.com/go-chi/chi/v5"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{service: coresecret.NewSecretService()}
}

type RequestHandler struct {
	service *coresecret.SecretService
}

func (h *RequestHandler) GetSecretList(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.List(r.URL.Query().Get("namespace"))
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "list failed: "+err.Error(), nil)
		return
	}
	res := make([]SecretSummary, 0, len(list))
	for _, secret := range list {
		res = append(res, sanitize(secret))
	}
	apimodel.RespondSuccess(w, http.StatusOK, "secret list", res)
}

func (h *RequestHandler) GetSecret(w http.ResponseWriter, r *http.Request) {
	idOrName := chi.URLParam(r, "idOrName")
	info, err := h.service.Get(idOrName, r.URL.Query().Get("namespace"))
	if err != nil {
		apimodel.RespondFail(w, http.StatusNotFound, "get failed: "+err.Error(), nil)
		return
	}
	apimodel.RespondSuccess(w, http.StatusOK, "secret detail", sanitize(info))
}

func (h *RequestHandler) RemoveSecret(w http.ResponseWriter, r *http.Request) {
	idOrName := chi.URLParam(r, "idOrName")
	info, err := h.service.Remove(idOrName, r.URL.Query().Get("namespace"))
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "remove failed: "+err.Error(), nil)
		return
	}
	apimodel.RespondSuccess(w, http.StatusOK, "secret removed", sanitize(info))
}

func sanitize(info sec.SecretInfo) SecretSummary {
	keys := make([]string, 0, len(info.Data))
	for k := range info.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return SecretSummary{
		SecretId:  info.SecretId,
		Name:      info.Name,
		Namespace: info.Namespace,
		Type:      info.Type,
		Keys:      keys,
		CreatedAt: info.CreatedAt.Format(time.RFC3339),
	}
}
