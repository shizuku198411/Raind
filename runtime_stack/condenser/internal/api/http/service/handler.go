package service

import (
	"io"
	"net/http"
	"time"

	apimodel "condenser/internal/api/http/utils"
	coreService "condenser/internal/core/service"
	"condenser/internal/store/ssm"
	"condenser/internal/utils"

	"github.com/go-chi/chi/v5"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{
		ssmHandler: ssm.NewSsmManager(ssm.NewSsmStore(utils.SsmStorePath)),
	}
}

type RequestHandler struct {
	ssmHandler ssm.SsmHandler
}

// CreateService godoc
// @Summary create service
// @Description create a L4 service
// @Tags services
// @Accept text/plain
// @Produce json
// @Param request body string true "Service Manifest (yaml)"
// @Success 201 {object} apimodel.ApiResponse
// @Router /v1/services [post]
func (h *RequestHandler) CreateService(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "read body failed: "+err.Error(), CreateServiceResponse{ServiceId: ""})
		return
	}
	manifest, err := coreService.DecodeK8sServiceManifest(body)
	if err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid yaml: "+err.Error(), CreateServiceResponse{ServiceId: ""})
		return
	}
	if manifest.Name == "" || manifest.Namespace == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "name and namespace are required", CreateServiceResponse{ServiceId: ""})
		return
	}
	if h.ssmHandler.IsNameAlreadyUsed(manifest.Name, manifest.Namespace) {
		apimodel.RespondFail(w, http.StatusBadRequest, "name already used by other service", CreateServiceResponse{ServiceId: ""})
		return
	}

	serviceId := utils.NewUlid()
	if err := h.ssmHandler.StoreService(serviceId, ssm.ServiceInfo{
		Name:      manifest.Name,
		Namespace: manifest.Namespace,
		Selector:  manifest.Selector,
		Ports:     manifest.Ports,
	}); err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "store failed: "+err.Error(), CreateServiceResponse{ServiceId: ""})
		return
	}

	apimodel.RespondSuccess(w, http.StatusCreated, "service created", CreateServiceResponse{ServiceId: serviceId})
}

// GetServiceList godoc
// @Summary list services
// @Description list services
// @Tags services
// @Produce json
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/services [get]
func (h *RequestHandler) GetServiceList(w http.ResponseWriter, r *http.Request) {
	list, err := h.ssmHandler.GetServiceList()
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "list failed: "+err.Error(), nil)
		return
	}
	res := make([]ServiceSummary, 0, len(list))
	for _, s := range list {
		var ports []ServicePort
		for _, p := range s.Ports {
			ports = append(ports, ServicePort{
				Port:       p.Port,
				TargetPort: p.TargetPort,
				Protocol:   p.Protocol,
			})
		}
		res = append(res, ServiceSummary{
			ServiceId: s.ServiceId,
			Name:      s.Name,
			Namespace: s.Namespace,
			Ports:     ports,
			CreatedAt: s.CreatedAt.Format(time.RFC3339),
		})
	}
	apimodel.RespondSuccess(w, http.StatusOK, "service list", res)
}

// GetServiceById godoc
// @Summary get service detail
// @Description get service detail
// @Tags services
// @Param serviceId path string true "Service ID"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/services/{serviceId} [get]
func (h *RequestHandler) GetServiceById(w http.ResponseWriter, r *http.Request) {
	serviceId := chi.URLParam(r, "serviceId")
	if serviceId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing serviceId", nil)
		return
	}
	info, err := h.ssmHandler.GetServiceById(serviceId)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "get failed: "+err.Error(), nil)
		return
	}
	apimodel.RespondSuccess(w, http.StatusOK, "service detail", info)
}

// RemoveService godoc
// @Summary remove service
// @Description remove service
// @Tags services
// @Param serviceId path string true "Service ID"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/services/{serviceId} [delete]
func (h *RequestHandler) RemoveService(w http.ResponseWriter, r *http.Request) {
	serviceId := chi.URLParam(r, "serviceId")
	if serviceId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing serviceId", nil)
		return
	}
	if err := h.ssmHandler.RemoveService(serviceId); err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "remove failed: "+err.Error(), nil)
		return
	}
	apimodel.RespondSuccess(w, http.StatusOK, "service removed", map[string]string{"serviceId": serviceId})
}
