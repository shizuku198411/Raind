package namespace

import (
	"net/http"
	apimodel "raind/internal/condenser/api/http/utils"
	corenamespace "raind/internal/condenser/core/namespace"

	"github.com/go-chi/chi/v5"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{
		serviceHandler: corenamespace.NewNamespaceService(),
	}
}

type RequestHandler struct {
	serviceHandler corenamespace.NamespaceServiceHandler
}

func (h *RequestHandler) GetNamespaceList(w http.ResponseWriter, r *http.Request) {
	list, err := h.serviceHandler.List()
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "get namespace list failed: "+err.Error(), nil)
		return
	}
	apimodel.RespondSuccess(w, http.StatusOK, "namespace list", list)
}

func (h *RequestHandler) CreateNamespace(w http.ResponseWriter, r *http.Request) {
	var req CreateNamespaceRequest
	if err := apimodel.DecodeRequestBody(r, &req); err != nil {
		apimodel.RespondFail(w, apimodel.DecodeErrorStatus(err), "invalid json: "+err.Error(), nil)
		return
	}
	info, err := h.serviceHandler.Create(corenamespace.ServiceCreateModel{
		Name:        req.Name,
		Network:     req.Network,
		Labels:      req.Labels,
		Annotations: req.Annotations,
	})
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "create namespace failed: "+err.Error(), nil)
		return
	}
	apimodel.RespondSuccess(w, http.StatusCreated, "namespace created", info)
}

func (h *RequestHandler) GetNamespace(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing namespace", nil)
		return
	}
	info, err := h.serviceHandler.Get(name)
	if err != nil {
		apimodel.RespondFail(w, http.StatusNotFound, err.Error(), nil)
		return
	}
	apimodel.RespondSuccess(w, http.StatusOK, "namespace", info)
}

func (h *RequestHandler) DeleteNamespace(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if name == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing namespace", nil)
		return
	}
	deleted, err := h.serviceHandler.Remove(corenamespace.ServiceRemoveModel{Name: name})
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "delete namespace failed: "+err.Error(), nil)
		return
	}
	apimodel.RespondSuccess(w, http.StatusOK, "delete namespace: "+deleted+" completed", nil)
}
