package container

import (
	"condenser/internal/core/container"
	"condenser/internal/store/csm"
	"condenser/internal/utils"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"condenser/internal/api/http/logger"
	apimodel "condenser/internal/api/http/utils"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{
		serviceHandler: container.NewContaierService(),
		csmHandler:     csm.NewCsmManager(csm.NewCsmStore(utils.CsmStorePath)),
	}
}

type RequestHandler struct {
	serviceHandler container.ContainerServiceHandler
	csmHandler     csm.CsmHandler
}

// CreateContainer godoc
// @Summary Create a container
// @Description create a new container
// @Tags containers
// @Accept json
// @Produce json
// @Param request body CreateContainerRequest true "Container Spec"
// @Success 201 {object} apimodel.ApiResponse
// @Router /v1/containers [post]
func (h *RequestHandler) CreateContainer(w http.ResponseWriter, r *http.Request) {
	// decode request
	var req CreateContainerRequest
	if err := apimodel.DecodeRequestBody(r, &req); err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), CreateContainerResponse{Id: ""})
		return
	}

	// set log: target
	logger.SetTarget(r.Context(), logger.Target{
		ContainerName: req.Name,
		ImageRef:      req.Image,
		Command:       req.Command,
		Port:          req.Port,
		Mount:         req.Mount,
		Network:       req.Network,
		Tty:           req.Tty,
	})

	// service: create
	result, err := h.serviceHandler.Create(
		container.ServiceCreateModel{
			Image:   req.Image,
			Command: req.Command,
			Port:    req.Port,
			Mount:   req.Mount,
			Env:     req.Env,
			Network: req.Network,
			Tty:     req.Tty,
			Name:    req.Name,
			PodId:   req.PodId,
		},
	)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "service failed: "+err.Error(), CreateContainerResponse{Id: ""})
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "cotainer created", CreateContainerResponse{Id: result})
}

// StartContainer godoc
// @Summary start a container
// @Description start an exitsting container
// @Tags containers
// @Param containerId path string true "Container ID"
// @Param request body StartContainerRequest true "Start Options"
// @Success 201 {object} apimodel.ApiResponse
// @Router /v1/containers/{containerId}/actions/start [post]
func (h *RequestHandler) StartContainer(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if containerId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing container Id", StartContainerResponse{Id: ""})
		return
	}

	// decode request
	var req StartContainerRequest
	if err := apimodel.DecodeRequestBody(r, &req); err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), StartContainerResponse{Id: containerId})
		return
	}

	// set log: target
	log_containerId, log_containerName, _ := h.csmHandler.GetContainerIdAndName(containerId)
	logger.SetTarget(r.Context(), logger.Target{
		ContainerId:   log_containerId,
		ContainerName: log_containerName,
		Tty:           req.Tty,
	})

	// service: start
	result, err := h.serviceHandler.Start(
		container.ServiceStartModel{
			ContainerId: containerId,
			Tty:         req.Tty,
		},
	)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "service failed: "+err.Error(), StartContainerResponse{Id: containerId})
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "container started", StartContainerResponse{Id: result})
}

// StopContainer godoc
// @Summary stop a container
// @Description stop an exitsting container
// @Tags containers
// @Param containerId path string true "Container ID"
// @Success 201 {object} apimodel.ApiResponse
// @Router /v1/containers/{containerId}/actions/stop [post]
func (h *RequestHandler) StopContainer(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if containerId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing containerId", StopContainerResponse{Id: ""})
		return
	}

	// set log: target
	log_containerId, log_containerName, _ := h.csmHandler.GetContainerIdAndName(containerId)
	logger.SetTarget(r.Context(), logger.Target{
		ContainerId:   log_containerId,
		ContainerName: log_containerName,
	})

	// service: stop
	result, err := h.serviceHandler.Stop(
		container.ServiceStopModel{
			ContainerId: containerId,
		},
	)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "service failed: "+err.Error(), StopContainerResponse{Id: containerId})
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "container stopped", StopContainerResponse{Id: result})
}

// ExecContainer godoc
// @Summary exec a container
// @Description execute command inside an exitsting container
// @Tags containers
// @Param containerId path string true "Container ID"
// @Param request body ExecContainerRequest true "Execute Options"
// @Success 201 {object} apimodel.ApiResponse
// @Router /v1/containers/{containerId}/actions/exec [post]
func (h *RequestHandler) ExecContainer(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if containerId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing containerId", ExecContainerResponse{Id: ""})
		return
	}

	// decode request
	var req ExecContainerRequest
	if err := apimodel.DecodeRequestBody(r, &req); err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), ExecContainerResponse{Id: containerId})
		return
	}

	// set log: target
	log_containerId, log_containerName, _ := h.csmHandler.GetContainerIdAndName(containerId)
	logger.SetTarget(r.Context(), logger.Target{
		ContainerId:   log_containerId,
		ContainerName: log_containerName,
		Command:       req.Command,
		Tty:           req.Tty,
	})

	// service: exec
	err := h.serviceHandler.Exec(container.ServiceExecModel{
		ContainerId: containerId,
		Tty:         req.Tty,
		Entrypoint:  req.Command,
	})
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "service failed: "+err.Error(), nil)
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "container executed", ExecContainerResponse{Id: containerId})
}

// DeleteContainer godoc
// @Summary delete a container
// @Description delete an exitsting container
// @Tags containers
// @Param containerId path string true "Container ID"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/containers/{containerId}/actions/delete [delete]
func (h *RequestHandler) DeleteContainer(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if containerId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing containerId", DeleteContainerResponse{Id: ""})
		return
	}

	// set log: target
	log_containerId, log_containerName, _ := h.csmHandler.GetContainerIdAndName(containerId)
	logger.SetTarget(r.Context(), logger.Target{
		ContainerId:   log_containerId,
		ContainerName: log_containerName,
	})

	// service: delete
	result, err := h.serviceHandler.Delete(
		container.ServiceDeleteModel{
			ContainerId: containerId,
		},
	)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "service failed: "+err.Error(), DeleteContainerResponse{Id: containerId})
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "container deleted", DeleteContainerResponse{Id: result})
}

// GetContainerList godoc
// @Summary get container list
// @Description get all container list
// @Tags containers
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/containers [get]
func (h *RequestHandler) GetContainerList(w http.ResponseWriter, r *http.Request) {
	// service: get container list
	containerList, err := h.serviceHandler.GetContainerList()
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "retrieve container list failed: "+err.Error(), nil)
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "retrieve container list success", containerList)
}

// GetContainerById godoc
// @Summary get container info
// @Description get an exitsting container info
// @Tags containers
// @Param containerId path string true "Container ID"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/containers/{containerId} [get]
func (h *RequestHandler) GetContainerById(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if containerId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing container Id", StartContainerResponse{Id: ""})
		return
	}

	// set log: target
	log_containerId, log_containerName, _ := h.csmHandler.GetContainerIdAndName(containerId)
	logger.SetTarget(r.Context(), logger.Target{
		ContainerId:   log_containerId,
		ContainerName: log_containerName,
	})

	// service: get container by id
	containerInfo, err := h.serviceHandler.GetContainerById(containerId)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, err.Error(), nil)
		return
	}

	// encode response
	apimodel.RespondSuccess(w, http.StatusOK, "retrieve container info success", containerInfo)
}

// GetContainerLog godoc
// @Summary get container log
// @Description get container log
// @Tags containers
// @Param containerId path string true "Container ID"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/containers/{containerId}/log [get]
func (h *RequestHandler) GetContainerLog(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if containerId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing container Id", nil)
		return
	}
	query := r.URL.Query()

	if s := query.Get("tail_lines"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			apimodel.RespondFail(w, http.StatusBadRequest, "invalid tail_lines", nil)
			return
		}
		data, err := h.serviceHandler.GetLogWithTailLines(containerId, n)
		if err != nil {
			apimodel.RespondFail(w, http.StatusInternalServerError, "tail failed: "+err.Error(), nil)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write(data)
		return
	}
}

// GetContainerStats godoc
// @Summary get container stats
// @Description get container stats
// @Tags containers
// @Param containerId path string true "Container ID"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/containers/{containerId}/stats [get]
func (h *RequestHandler) GetContainerStats(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if containerId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing container Id", nil)
		return
	}

	stats, err := h.serviceHandler.GetContainerStats(containerId)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "retrieve container stats failed: "+err.Error(), nil)
		return
	}

	apimodel.RespondSuccess(w, http.StatusOK, "retrieve container stats success", stats)
}

// ListContainerStats godoc
// @Summary list container stats
// @Description list container stats
// @Tags containers
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/containers/stats [get]
func (h *RequestHandler) ListContainerStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.serviceHandler.ListContainerStats()
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "retrieve container stats failed: "+err.Error(), nil)
		return
	}

	apimodel.RespondSuccess(w, http.StatusOK, "retrieve container stats success", stats)
}

// GetContainerLogPath godoc
// @Summary get container log path
// @Description get container log path
// @Tags containers
// @Param containerId path string true "Container ID"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/containers/{containerId}/logpath [get]
func (h *RequestHandler) GetContainerLogPath(w http.ResponseWriter, r *http.Request) {
	containerId := chi.URLParam(r, "containerId")
	if containerId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing container Id", nil)
		return
	}

	logPath, err := h.serviceHandler.GetContainerLogPath(containerId)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "log path: "+err.Error(), nil)
		return
	}

	apimodel.RespondSuccess(w, http.StatusOK, "log path", logPath)
	return
}
