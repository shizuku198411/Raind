package pod

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	apimodel "condenser/internal/api/http/utils"
	"condenser/internal/core/container"
	"condenser/internal/core/pod"
	coreService "condenser/internal/core/service"
	"condenser/internal/store/psm"
	"condenser/internal/store/ssm"
	"condenser/internal/utils"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{
		serviceHandler:   pod.NewPodService(),
		containerHandler: container.NewContaierService(),
		psmHandler:       psm.NewPsmManager(psm.NewPsmStore(utils.PsmStorePath)),
		ssmHandler:       ssm.NewSsmManager(ssm.NewSsmStore(utils.SsmStorePath)),
	}
}

type RequestHandler struct {
	serviceHandler   pod.PodServiceHandler
	containerHandler container.ContainerServiceHandler
	psmHandler       psm.PsmHandler
	ssmHandler       ssm.SsmHandler
}

// CreatePod godoc
// @Summary create pod sandbox
// @Description create a pod sandbox (no container start)
// @Tags pods
// @Accept json
// @Produce json
// @Param request body CreatePodRequest true "Pod Spec"
// @Success 201 {object} apimodel.ApiResponse
// @Router /v1/pods [post]
func (h *RequestHandler) CreatePod(w http.ResponseWriter, r *http.Request) {
	var req CreatePodRequest
	if err := apimodel.DecodeRequestBody(r, &req); err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), CreatePodResponse{PodId: ""})
		return
	}

	podId, err := h.serviceHandler.Create(pod.ServiceCreateModel{
		Name:        req.Name,
		Namespace:   req.Namespace,
		UID:         req.UID,
		NetworkNS:   req.NetworkNS,
		IPCNS:       req.IPCNS,
		UTSNS:       req.UTSNS,
		UserNS:      req.UserNS,
		Labels:      req.Labels,
		Annotations: req.Annotations,
		Containers: func() []psm.ContainerTemplateSpec {
			if len(req.Containers) == 0 {
				return nil
			}
			specs := make([]psm.ContainerTemplateSpec, 0, len(req.Containers))
			for _, c := range req.Containers {
				specs = append(specs, psm.ContainerTemplateSpec{
					Name:    c.Name,
					Image:   c.Image,
					Command: c.Command,
					Port:    c.Port,
					Mount:   c.Mount,
					Env:     c.Env,
					Network: c.Network,
					Tty:     c.Tty,
				})
			}
			return specs
		}(),
	})
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "create pod failed: "+err.Error(), CreatePodResponse{PodId: ""})
		return
	}

	apimodel.RespondSuccess(w, http.StatusOK, "pod created", CreatePodResponse{PodId: podId})
}

// ApplyPodYaml godoc
// @Summary apply pod/replicaset manifest
// @Description apply kubectl-compatible yaml manifest
// @Tags pods
// @Accept text/plain
// @Produce json
// @Success 201 {object} apimodel.ApiResponse
// @Router /v1/resource/apply [post]
func (h *RequestHandler) ApplyPodYaml(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid body: "+err.Error(), nil)
		return
	}

	var results []ApplyPodResult
	var serviceResults []ApplyServiceResult

	dec := yaml.NewDecoder(bytes.NewReader(body))
	for {
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			apimodel.RespondFail(w, http.StatusBadRequest, "invalid yaml: "+err.Error(), nil)
			return
		}
		if len(raw) == 0 {
			continue
		}
		kind, _ := raw["kind"].(string)
		if kind == "" {
			apimodel.RespondFail(w, http.StatusBadRequest, "kind is required", nil)
			return
		}
		rawBytes, err := yaml.Marshal(raw)
		if err != nil {
			apimodel.RespondFail(w, http.StatusBadRequest, "invalid yaml: "+err.Error(), nil)
			return
		}

		switch kind {
		case "Service":
			manifest, err := coreService.DecodeK8sServiceManifest(rawBytes)
			if err != nil {
				apimodel.RespondFail(w, http.StatusBadRequest, "invalid yaml: "+err.Error(), nil)
				return
			}
			if manifest.Name == "" || manifest.Namespace == "" {
				apimodel.RespondFail(w, http.StatusBadRequest, "name and namespace are required", nil)
				return
			}
			if h.ssmHandler.IsNameAlreadyUsed(manifest.Name, manifest.Namespace) {
				apimodel.RespondFail(w, http.StatusBadRequest, "name already used by other service", nil)
				return
			}
			serviceId := utils.NewUlid()
			if err := h.ssmHandler.StoreService(serviceId, ssm.ServiceInfo{
				Name:      manifest.Name,
				Namespace: manifest.Namespace,
				Selector:  manifest.Selector,
				Ports:     manifest.Ports,
			}); err != nil {
				apimodel.RespondFail(w, http.StatusInternalServerError, "service store failed: "+err.Error(), nil)
				return
			}
			serviceResults = append(serviceResults, ApplyServiceResult{
				ServiceId: serviceId,
				Name:      manifest.Name,
				Namespace: manifest.Namespace,
			})
		case "Pod", "ReplicaSet":
			manifests, err := pod.DecodeK8sManifests(rawBytes)
			if err != nil || len(manifests) == 0 {
				apimodel.RespondFail(w, http.StatusBadRequest, "invalid yaml: "+err.Error(), nil)
				return
			}
			m := manifests[0]
			if m.Kind == "ReplicaSet" {
				templateId := utils.NewUlid()
				if err := h.psmHandler.StorePodTemplate(templateId, psm.PodTemplateSpec{
					Name:        m.Name,
					Namespace:   m.Namespace,
					Labels:      m.Labels,
					Annotations: m.Annotations,
					Containers:  m.Containers,
				}); err != nil {
					apimodel.RespondFail(w, http.StatusInternalServerError, "template store failed: "+err.Error(), nil)
					return
				}
				replicaSetId := utils.NewUlid()
				if err := h.psmHandler.StoreReplicaSet(replicaSetId, psm.ReplicaSetSpec{
					Name:       m.Name,
					Namespace:  m.Namespace,
					Replicas:   m.Replicas,
					TemplateId: templateId,
					Selector:   m.Selector,
				}); err != nil {
					apimodel.RespondFail(w, http.StatusInternalServerError, "replicaset store failed: "+err.Error(), nil)
					return
				}
				results = append(results, ApplyPodResult{
					ReplicaSetId: replicaSetId,
					Namespace:    m.Namespace,
					Name:         m.Name,
				})
				continue
			}

			podId, err := h.serviceHandler.Create(pod.ServiceCreateModel{
				Name:        m.Name,
				Namespace:   m.Namespace,
				Labels:      m.Labels,
				Annotations: m.Annotations,
				Containers:  m.Containers,
			})
			if err != nil {
				apimodel.RespondFail(w, http.StatusInternalServerError, "pod create failed: "+err.Error(), nil)
				return
			}

			var containerIds []string
			for _, c := range m.Containers {
				if c.Image == "" {
					continue
				}
				containerId, err := h.containerHandler.Create(container.ServiceCreateModel{
					Image:   c.Image,
					Command: c.Command,
					Port:    c.Port,
					Mount:   c.Mount,
					Env:     c.Env,
					Network: c.Network,
					Tty:     c.Tty,
					Name:    c.Name,
					PodId:   podId,
				})
				if err != nil {
					_, _ = h.serviceHandler.Remove(podId)
					apimodel.RespondFail(w, http.StatusInternalServerError, "container create failed: "+err.Error(), nil)
					return
				}
				containerIds = append(containerIds, containerId)
			}

			results = append(results, ApplyPodResult{
				PodId:        podId,
				Namespace:    m.Namespace,
				Name:         m.Name,
				ContainerIds: containerIds,
			})
		default:
			apimodel.RespondFail(w, http.StatusBadRequest, "unsupported kind: "+kind, nil)
			return
		}
	}

	apimodel.RespondSuccess(w, http.StatusCreated, "resources applied", ApplyPodResponse{Pods: results, Services: serviceResults})
}

// DeleteResourceYaml godoc
// @Summary delete resources by manifest
// @Description delete resources defined in kubectl-compatible yaml manifest
// @Tags resources
// @Accept text/plain
// @Produce json
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/resource/delete [post]
func (h *RequestHandler) DeleteResourceYaml(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid body: "+err.Error(), nil)
		return
	}

	var podResults []DeletePodResult
	var rsResults []DeleteReplicaSetResult
	var svcResults []DeleteServiceResult

	dec := yaml.NewDecoder(bytes.NewReader(body))
	for {
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			apimodel.RespondFail(w, http.StatusBadRequest, "invalid yaml: "+err.Error(), nil)
			return
		}
		if len(raw) == 0 {
			continue
		}
		kind, _ := raw["kind"].(string)
		if kind == "" {
			apimodel.RespondFail(w, http.StatusBadRequest, "kind is required", nil)
			return
		}
		rawBytes, err := yaml.Marshal(raw)
		if err != nil {
			apimodel.RespondFail(w, http.StatusBadRequest, "invalid yaml: "+err.Error(), nil)
			return
		}

		switch kind {
		case "Service":
			manifest, err := coreService.DecodeK8sServiceManifest(rawBytes)
			if err != nil {
				apimodel.RespondFail(w, http.StatusBadRequest, "invalid yaml: "+err.Error(), nil)
				return
			}
			list, err := h.ssmHandler.GetServiceList()
			if err != nil {
				apimodel.RespondFail(w, http.StatusInternalServerError, "list failed: "+err.Error(), nil)
				return
			}
			var removed bool
			for _, s := range list {
				if s.Name != manifest.Name || s.Namespace != manifest.Namespace {
					continue
				}
				if err := h.ssmHandler.RemoveService(s.ServiceId); err != nil {
					apimodel.RespondFail(w, http.StatusInternalServerError, "remove failed: "+err.Error(), nil)
					return
				}
				svcResults = append(svcResults, DeleteServiceResult{
					ServiceId: s.ServiceId,
					Name:      s.Name,
					Namespace: s.Namespace,
				})
				removed = true
			}
			if !removed {
				apimodel.RespondFail(w, http.StatusNotFound, "service not found", nil)
				return
			}
		case "ReplicaSet":
			manifests, err := pod.DecodeK8sManifests(rawBytes)
			if err != nil || len(manifests) == 0 {
				apimodel.RespondFail(w, http.StatusBadRequest, "invalid yaml: "+err.Error(), nil)
				return
			}
			m := manifests[0]
			list, err := h.psmHandler.GetReplicaSetList()
			if err != nil {
				apimodel.RespondFail(w, http.StatusInternalServerError, "list failed: "+err.Error(), nil)
				return
			}
			var removed bool
			for _, rs := range list {
				if rs.Spec.Name != m.Name || rs.Spec.Namespace != m.Namespace {
					continue
				}
				if err := h.removeReplicaSetById(rs.ReplicaSetId); err != nil {
					apimodel.RespondFail(w, http.StatusInternalServerError, "remove failed: "+err.Error(), nil)
					return
				}
				rsResults = append(rsResults, DeleteReplicaSetResult{
					ReplicaSetId: rs.ReplicaSetId,
					Name:         rs.Spec.Name,
					Namespace:    rs.Spec.Namespace,
				})
				removed = true
			}
			if !removed {
				apimodel.RespondFail(w, http.StatusNotFound, "replicaset not found", nil)
				return
			}
		case "Pod":
			manifests, err := pod.DecodeK8sManifests(rawBytes)
			if err != nil || len(manifests) == 0 {
				apimodel.RespondFail(w, http.StatusBadRequest, "invalid yaml: "+err.Error(), nil)
				return
			}
			m := manifests[0]
			list, err := h.psmHandler.GetPodList()
			if err != nil {
				apimodel.RespondFail(w, http.StatusInternalServerError, "list failed: "+err.Error(), nil)
				return
			}
			var removed bool
			for _, p := range list {
				if p.Name != m.Name || p.Namespace != m.Namespace {
					continue
				}
				if _, err := h.serviceHandler.Remove(p.PodId); err != nil {
					apimodel.RespondFail(w, http.StatusInternalServerError, "remove failed: "+err.Error(), nil)
					return
				}
				podResults = append(podResults, DeletePodResult{
					PodId:     p.PodId,
					Name:      p.Name,
					Namespace: p.Namespace,
				})
				removed = true
			}
			if !removed {
				apimodel.RespondFail(w, http.StatusNotFound, "pod not found", nil)
				return
			}
		default:
			apimodel.RespondFail(w, http.StatusBadRequest, "unsupported kind: "+kind, nil)
			return
		}
	}

	apimodel.RespondSuccess(w, http.StatusOK, "resources deleted", DeleteResourcesResponse{
		Pods:        podResults,
		ReplicaSets: rsResults,
		Services:    svcResults,
	})
}

func (h *RequestHandler) removeReplicaSetById(replicaSetId string) error {
	rs, err := h.psmHandler.GetReplicaSet(replicaSetId)
	if err != nil {
		return err
	}
	if err := h.psmHandler.RemoveReplicaSet(replicaSetId); err != nil {
		return err
	}
	// delete pods and template (best-effort)
	pods, err := h.psmHandler.GetPodList()
	if err == nil {
		for _, p := range pods {
			if p.TemplateId == rs.Spec.TemplateId || labelsMatch(rs.Spec.Selector, p.Labels, rs.Spec.Namespace, p.Namespace) {
				_, _ = h.serviceHandler.Remove(p.PodId)
			}
		}
	}
	inUse, err := h.psmHandler.IsTemplateReferenced(rs.Spec.TemplateId)
	if err == nil && !inUse {
		_ = h.psmHandler.RemovePodTemplate(rs.Spec.TemplateId)
	}
	return nil
}

func labelsMatch(selector, labels map[string]string, selectorNamespace, podNamespace string) bool {
	if selectorNamespace != "" && selectorNamespace != podNamespace {
		return false
	}
	if len(selector) == 0 {
		return false
	}
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}

func (h *RequestHandler) getTemplateContainerCount(templateId string) int {
	if templateId == "" {
		return 0
	}
	tmpl, err := h.psmHandler.GetPodTemplate(templateId)
	if err != nil {
		return 0
	}
	return len(tmpl.Spec.Containers)
}

func (h *RequestHandler) getPodContainerCounts(podId string, desiredFallback int) (int, int, error) {
	containers, err := h.containerHandler.GetContainersByPodId(podId)
	if err != nil {
		return desiredFallback, 0, err
	}
	running := 0
	nonInfra := 0
	for _, c := range containers {
		if strings.HasPrefix(c.Name, utils.PodInfraContainerNamePrefix) {
			continue
		}
		nonInfra++
		if c.State == "running" {
			running++
		}
	}
	desired := desiredFallback
	if desired == 0 {
		desired = nonInfra
	}
	return desired, running, nil
}

// ScaleReplicaSet godoc
// @Summary scale replica set
// @Description scale replica set replicas
// @Tags replicasets
// @Accept json
// @Produce json
// @Param replicaSetId path string true "ReplicaSet ID"
// @Param request body ScaleReplicaSetRequest true "Scale Options"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/replicasets/{replicaSetId}/actions/scale [post]
func (h *RequestHandler) ScaleReplicaSet(w http.ResponseWriter, r *http.Request) {
	replicaSetId := chi.URLParam(r, "replicaSetId")
	if replicaSetId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing replicaSetId", ScaleReplicaSetResponse{ReplicaSetId: "", Replicas: 0})
		return
	}

	var req ScaleReplicaSetRequest
	if err := apimodel.DecodeRequestBody(r, &req); err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid json: "+err.Error(), ScaleReplicaSetResponse{ReplicaSetId: replicaSetId})
		return
	}
	if req.Replicas < 0 {
		apimodel.RespondFail(w, http.StatusBadRequest, "replicas must be >= 0", ScaleReplicaSetResponse{ReplicaSetId: replicaSetId})
		return
	}

	if err := h.psmHandler.UpdateReplicaSetReplicas(replicaSetId, req.Replicas); err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "scale failed: "+err.Error(), ScaleReplicaSetResponse{ReplicaSetId: replicaSetId})
		return
	}

	apimodel.RespondSuccess(w, http.StatusOK, "replicaset scaled", ScaleReplicaSetResponse{ReplicaSetId: replicaSetId, Replicas: req.Replicas})
}

// GetReplicaSetList godoc
// @Summary list replica sets
// @Description list replica sets
// @Tags replicasets
// @Produce json
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/replicasets [get]
func (h *RequestHandler) GetReplicaSetList(w http.ResponseWriter, r *http.Request) {
	list, err := h.psmHandler.GetReplicaSetList()
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "list failed: "+err.Error(), nil)
		return
	}
	pods, err := h.psmHandler.GetPodList()
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "list pods failed: "+err.Error(), nil)
		return
	}
	res := make([]ReplicaSetSummary, 0, len(list))
	for _, rs := range list {
		templateCount := h.getTemplateContainerCount(rs.Spec.TemplateId)
		current := 0
		ready := 0
		for _, p := range pods {
			if !labelsMatch(rs.Spec.Selector, p.Labels, rs.Spec.Namespace, p.Namespace) && p.TemplateId != rs.Spec.TemplateId {
				continue
			}
			current++
			desired, running, err := h.getPodContainerCounts(p.PodId, templateCount)
			if err != nil {
				continue
			}
			if desired > 0 && running == desired {
				ready++
			}
		}
		res = append(res, ReplicaSetSummary{
			ReplicaSetId: rs.ReplicaSetId,
			Name:         rs.Spec.Name,
			Namespace:    rs.Spec.Namespace,
			Replicas:     rs.Spec.Replicas,
			Desired:      rs.Spec.Replicas,
			Current:      current,
			Ready:        ready,
			TemplateId:   rs.Spec.TemplateId,
			Selector:     rs.Spec.Selector,
			CreatedAt:    rs.CreatedAt.Format(time.RFC3339),
		})
	}
	apimodel.RespondSuccess(w, http.StatusOK, "replicaset list", res)
}

// GetReplicaSetById godoc
// @Summary get replica set detail
// @Description get replica set detail
// @Tags replicasets
// @Param replicaSetId path string true "ReplicaSet ID"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/replicasets/{replicaSetId} [get]
func (h *RequestHandler) GetReplicaSetById(w http.ResponseWriter, r *http.Request) {
	replicaSetId := chi.URLParam(r, "replicaSetId")
	if replicaSetId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing replicaSetId", nil)
		return
	}
	rs, err := h.psmHandler.GetReplicaSet(replicaSetId)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "get failed: "+err.Error(), nil)
		return
	}
	template, err := h.psmHandler.GetPodTemplate(rs.Spec.TemplateId)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "template lookup failed: "+err.Error(), nil)
		return
	}
	pods, err := h.psmHandler.GetPodList()
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "list pods failed: "+err.Error(), nil)
		return
	}
	current := 0
	ready := 0
	templateCount := len(template.Spec.Containers)
	for _, p := range pods {
		if !labelsMatch(rs.Spec.Selector, p.Labels, rs.Spec.Namespace, p.Namespace) && p.TemplateId != rs.Spec.TemplateId {
			continue
		}
		current++
		desired, running, err := h.getPodContainerCounts(p.PodId, templateCount)
		if err != nil {
			continue
		}
		if desired > 0 && running == desired {
			ready++
		}
	}
	apimodel.RespondSuccess(w, http.StatusOK, "replicaset detail", ReplicaSetDetail{
		ReplicaSetId: rs.ReplicaSetId,
		Name:         rs.Spec.Name,
		Namespace:    rs.Spec.Namespace,
		Replicas:     rs.Spec.Replicas,
		Desired:      rs.Spec.Replicas,
		Current:      current,
		Ready:        ready,
		Selector:     rs.Spec.Selector,
		Template:     template.Spec,
		CreatedAt:    rs.CreatedAt.Format(time.RFC3339),
	})
}

// RemoveReplicaSet godoc
// @Summary remove replica set
// @Description remove replica set
// @Tags replicasets
// @Param replicaSetId path string true "ReplicaSet ID"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/replicasets/{replicaSetId} [delete]
func (h *RequestHandler) RemoveReplicaSet(w http.ResponseWriter, r *http.Request) {
	replicaSetId := chi.URLParam(r, "replicaSetId")
	if replicaSetId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing replicaSetId", nil)
		return
	}
	rs, err := h.psmHandler.GetReplicaSet(replicaSetId)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "get failed: "+err.Error(), nil)
		return
	}
	if err := h.psmHandler.RemoveReplicaSet(replicaSetId); err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "remove failed: "+err.Error(), nil)
		return
	}
	// delete pods and template (best-effort)
	pods, err := h.psmHandler.GetPodList()
	if err == nil {
		for _, p := range pods {
			if p.TemplateId == rs.Spec.TemplateId {
				_, _ = h.serviceHandler.Remove(p.PodId)
			}
		}
	}
	inUse, err := h.psmHandler.IsTemplateReferenced(rs.Spec.TemplateId)
	if err == nil && !inUse {
		_ = h.psmHandler.RemovePodTemplate(rs.Spec.TemplateId)
	}

	apimodel.RespondSuccess(w, http.StatusOK, "replicaset removed", map[string]string{"replicaSetId": replicaSetId})
}

// StartPod godoc
// @Summary start pod sandbox
// @Description start a pod sandbox
// @Tags pods
// @Param podId path string true "Pod ID"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/pods/{podId}/actions/start [post]
func (h *RequestHandler) StartPod(w http.ResponseWriter, r *http.Request) {
	podId := chi.URLParam(r, "podId")
	if podId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing podId", StartPodResponse{PodId: ""})
		return
	}

	result, err := h.serviceHandler.Start(podId)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "start pod failed: "+err.Error(), StartPodResponse{PodId: podId})
		return
	}

	apimodel.RespondSuccess(w, http.StatusOK, "pod started", StartPodResponse{PodId: result})
}

// StopPod godoc
// @Summary stop pod sandbox
// @Description stop a pod sandbox
// @Tags pods
// @Param podId path string true "Pod ID"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/pods/{podId}/actions/stop [post]
func (h *RequestHandler) StopPod(w http.ResponseWriter, r *http.Request) {
	podId := chi.URLParam(r, "podId")
	if podId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing podId", StopPodResponse{PodId: ""})
		return
	}

	result, err := h.serviceHandler.Stop(podId)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "stop pod failed: "+err.Error(), StopPodResponse{PodId: podId})
		return
	}

	apimodel.RespondSuccess(w, http.StatusOK, "pod stopped", StopPodResponse{PodId: result})
}

// RemovePod godoc
// @Summary remove pod sandbox
// @Description remove a pod sandbox
// @Tags pods
// @Param podId path string true "Pod ID"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/pods/{podId} [delete]
func (h *RequestHandler) RemovePod(w http.ResponseWriter, r *http.Request) {
	podId := chi.URLParam(r, "podId")
	if podId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing podId", RemovePodResponse{PodId: ""})
		return
	}

	result, err := h.serviceHandler.Remove(podId)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "remove pod failed: "+err.Error(), RemovePodResponse{PodId: podId})
		return
	}

	apimodel.RespondSuccess(w, http.StatusOK, "pod removed", RemovePodResponse{PodId: result})
}

// GetPodList godoc
// @Summary list pods
// @Description list pod sandbox
// @Tags pods
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/pods [get]
func (h *RequestHandler) GetPodList(w http.ResponseWriter, r *http.Request) {
	podList, err := h.serviceHandler.GetPodList()
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "retrieve pod list failed: "+err.Error(), nil)
		return
	}
	apimodel.RespondSuccess(w, http.StatusOK, "retrieve pod list success", podList)
}

// GetPodById godoc
// @Summary get pod detail
// @Description get pod sandbox detail
// @Tags pods
// @Param podId path string true "Pod ID"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/pods/{podId} [get]
func (h *RequestHandler) GetPodById(w http.ResponseWriter, r *http.Request) {
	podId := chi.URLParam(r, "podId")
	if podId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing podId", nil)
		return
	}

	podInfo, err := h.serviceHandler.GetPodById(podId)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "retrieve pod failed: "+err.Error(), nil)
		return
	}

	apimodel.RespondSuccess(w, http.StatusOK, "retrieve pod success", podInfo)
}
