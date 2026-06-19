package pod

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	apimodel "raind/internal/condenser/api/http/utils"
	"raind/internal/condenser/core/container"
	corenamespace "raind/internal/condenser/core/namespace"
	"raind/internal/condenser/core/pod"
	coreresource "raind/internal/condenser/core/resource"
	"raind/internal/condenser/store/ism"
	"raind/internal/condenser/store/psm"
	"raind/internal/condenser/store/ssm"
	"raind/internal/condenser/utils"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"
)

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{
		serviceHandler:   pod.NewPodService(),
		containerHandler: container.NewContaierService(),
		namespaceHandler: corenamespace.NewNamespaceService(),
		psmHandler:       psm.NewPsmManager(psm.NewPsmStore(utils.PsmStorePath)),
		ssmHandler:       ssm.NewSsmManager(ssm.NewSsmStore(utils.SsmStorePath)),
		ismHandler:       ism.NewIsmManager(ism.NewIsmStore(utils.IsmStorePath)),
	}
}

type RequestHandler struct {
	serviceHandler   pod.PodServiceHandler
	containerHandler container.ContainerServiceHandler
	namespaceHandler corenamespace.NamespaceServiceHandler
	psmHandler       psm.PsmHandler
	ssmHandler       ssm.SsmHandler
	ismHandler       ism.IsmHandler
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
		apimodel.RespondFail(w, apimodel.DecodeErrorStatus(err), "invalid json: "+err.Error(), CreatePodResponse{PodId: ""})
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
		Rootless:    rootlessFromHostUsers(req.HostUsers),
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
					CapAdd:  c.CapAdd,
					CapDrop: c.CapDrop,
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
	body, err := apimodel.ReadLimitedBody(r.Body, apimodel.MaxManifestBodyBytes)
	if err != nil {
		apimodel.RespondFail(w, apimodel.DecodeErrorStatus(err), "invalid body: "+err.Error(), nil)
		return
	}

	result, err := coreresource.NewResourceService().Apply(body)
	if err != nil {
		apimodel.RespondFail(w, coreresource.ErrorStatus(err, http.StatusInternalServerError), err.Error(), nil)
		return
	}
	apimodel.RespondSuccess(w, http.StatusCreated, "resources applied", result)
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
	body, err := apimodel.ReadLimitedBody(r.Body, apimodel.MaxManifestBodyBytes)
	if err != nil {
		apimodel.RespondFail(w, apimodel.DecodeErrorStatus(err), "invalid body: "+err.Error(), nil)
		return
	}

	result, err := coreresource.NewResourceService().Delete(body)
	if err != nil {
		apimodel.RespondFail(w, coreresource.ErrorStatus(err, http.StatusInternalServerError), err.Error(), nil)
		return
	}
	apimodel.RespondSuccess(w, http.StatusOK, "resources deleted", result)
}

type namespaceManifest struct {
	APIVersion string        `yaml:"apiVersion"`
	Kind       string        `yaml:"kind"`
	Metadata   namespaceMeta `yaml:"metadata"`
}

func decodeNamespaceManifest(rawBytes []byte) (namespaceManifest, error) {
	var manifest namespaceManifest
	if err := yaml.Unmarshal(rawBytes, &manifest); err != nil {
		return namespaceManifest{}, err
	}
	if manifest.Metadata.Name == "" {
		return namespaceManifest{}, fmt.Errorf("namespace name is required")
	}
	return manifest, nil
}

type namespaceMeta struct {
	Name        string            `yaml:"name"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

func activeTLSHostsExcept(ingresses []ism.IngressInfo, excludedIngressId string) []string {
	var hosts []string
	for _, in := range ingresses {
		if in.IngressId == excludedIngressId {
			continue
		}
		hosts = append(hosts, in.TLSHosts...)
	}
	return hosts
}

func (h *RequestHandler) ensureResourceNameAvailable(name, namespace string) error {
	if name == "" || namespace == "" {
		return nil
	}
	if h.psmHandler.IsNameAlreadyUsed(name, namespace) {
		return errNameAlreadyUsed(name, namespace)
	}
	replicaSets, err := h.psmHandler.GetReplicaSetList()
	if err != nil {
		return err
	}
	for _, rs := range replicaSets {
		if rs.Spec.Name == name && rs.Spec.Namespace == namespace {
			return errNameAlreadyUsed(name, namespace)
		}
	}
	deployments, err := h.psmHandler.GetDeploymentList()
	if err != nil {
		return err
	}
	for _, deploy := range deployments {
		if deploy.Spec.Name == name && deploy.Spec.Namespace == namespace {
			return errNameAlreadyUsed(name, namespace)
		}
	}
	return nil
}

func errNameAlreadyUsed(name, namespace string) error {
	return fmt.Errorf("name already used by other resource: %s/%s", namespace, name)
}

func invalidYAMLErrorMessage(err error) string {
	if err == nil {
		return "invalid yaml"
	}
	return "invalid yaml: " + err.Error()
}

func (h *RequestHandler) resolveManifestNetworks(m *pod.PodManifest) error {
	networkName, err := h.namespaceHandler.ResolveNetwork(m.Namespace)
	if err != nil {
		return err
	}
	for i := range m.Containers {
		if m.Containers[i].Network == "" {
			m.Containers[i].Network = networkName
		}
	}
	return nil
}

func (h *RequestHandler) removeDeploymentById(deploymentId string) error {
	deploy, err := h.psmHandler.GetDeployment(deploymentId)
	if err != nil {
		return err
	}
	if err := h.psmHandler.RemoveDeployment(deploymentId); err != nil {
		return err
	}
	if deploy.Spec.ReplicaSetId != "" {
		if err := h.removeReplicaSetById(deploy.Spec.ReplicaSetId); err != nil && !strings.Contains(err.Error(), "not found") {
			return err
		}
	}
	inUse, err := h.psmHandler.IsTemplateReferenced(deploy.Spec.TemplateId)
	if err == nil && !inUse {
		_ = h.psmHandler.RemovePodTemplate(deploy.Spec.TemplateId)
	}
	return nil
}

func (h *RequestHandler) removeReplicaSetById(replicaSetId string) error {
	rs, err := h.psmHandler.GetReplicaSet(replicaSetId)
	if err != nil {
		return err
	}
	if err := h.psmHandler.RemoveReplicaSet(replicaSetId); err != nil {
		return err
	}
	inUse, err := h.psmHandler.IsTemplateReferenced(rs.Spec.TemplateId)
	if err == nil && !inUse {
		_ = h.psmHandler.RemovePodTemplate(rs.Spec.TemplateId)
	}
	if err := h.removePodsByTemplateId(rs.Spec.TemplateId); err != nil {
		return err
	}
	return nil
}

func (h *RequestHandler) removePodsByTemplateId(templateId string) error {
	pods, err := h.psmHandler.GetPodList()
	if err != nil {
		return err
	}
	for _, p := range pods {
		if p.TemplateId != templateId {
			continue
		}
		if _, err := h.serviceHandler.Remove(p.PodId); err != nil {
			return err
		}
	}
	return nil
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
		apimodel.RespondFail(w, apimodel.DecodeErrorStatus(err), "invalid json: "+err.Error(), ScaleReplicaSetResponse{ReplicaSetId: replicaSetId})
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
	namespaceFilter := r.URL.Query().Get("namespace")
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
		if namespaceFilter != "" && rs.Spec.Namespace != namespaceFilter {
			continue
		}
		templateCount := h.getTemplateContainerCount(rs.Spec.TemplateId)
		current := 0
		ready := 0
		for _, p := range pods {
			if !replicaSetOwnsPod(rs, p) {
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
			ReplicaSetId:       rs.ReplicaSetId,
			Name:               rs.Spec.Name,
			Namespace:          rs.Spec.Namespace,
			Replicas:           rs.Spec.Replicas,
			Desired:            rs.Spec.Replicas,
			Current:            current,
			Ready:              ready,
			TemplateId:         rs.Spec.TemplateId,
			Selector:           rs.Spec.Selector,
			ReconcileAttempt:   rs.ReconcileAttempt,
			LastReconcileError: rs.LastReconcileError,
			NextReconcileAt:    formatOptionalTime(rs.NextReconcileAt),
			CreatedAt:          rs.CreatedAt.Format(time.RFC3339),
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
		if !replicaSetOwnsPod(rs, p) {
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
		ReplicaSetId:       rs.ReplicaSetId,
		Name:               rs.Spec.Name,
		Namespace:          rs.Spec.Namespace,
		Replicas:           rs.Spec.Replicas,
		Desired:            rs.Spec.Replicas,
		Current:            current,
		Ready:              ready,
		Selector:           rs.Spec.Selector,
		Template:           template.Spec,
		ReconcileAttempt:   rs.ReconcileAttempt,
		LastReconcileError: rs.LastReconcileError,
		NextReconcileAt:    formatOptionalTime(rs.NextReconcileAt),
		CreatedAt:          rs.CreatedAt.Format(time.RFC3339),
	})
}

func formatOptionalTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
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
	if err := h.removeReplicaSetById(replicaSetId); err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "remove failed: "+err.Error(), nil)
		return
	}

	apimodel.RespondSuccess(w, http.StatusOK, "replicaset removed", map[string]string{"replicaSetId": replicaSetId})
}

func (h *RequestHandler) deploymentReplicaCounts(deploy psm.DeploymentInfo) (int, int, error) {
	if deploy.Spec.ReplicaSetId == "" {
		return 0, 0, nil
	}
	rs, err := h.psmHandler.GetReplicaSet(deploy.Spec.ReplicaSetId)
	if err != nil {
		return 0, 0, err
	}
	pods, err := h.psmHandler.GetPodList()
	if err != nil {
		return 0, 0, err
	}
	templateCount := h.getTemplateContainerCount(rs.Spec.TemplateId)
	current := 0
	ready := 0
	for _, p := range pods {
		if !replicaSetOwnsPod(rs, p) {
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
	return current, ready, nil
}

func replicaSetOwnsPod(rs psm.ReplicaSetInfo, p psm.PodInfo) bool {
	if p.OwnerKind == psm.OwnerKindReplicaSet || p.OwnerId != "" {
		return p.OwnerKind == psm.OwnerKindReplicaSet && p.OwnerId == rs.ReplicaSetId
	}
	return p.TemplateId == rs.Spec.TemplateId
}

// ScaleDeployment godoc
// @Summary scale deployment
// @Description scale deployment replicas
// @Tags deployments
// @Accept json
// @Produce json
// @Param deploymentId path string true "Deployment ID"
// @Param request body ScaleDeploymentRequest true "Scale Options"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/deployments/{deploymentId}/actions/scale [post]
func (h *RequestHandler) ScaleDeployment(w http.ResponseWriter, r *http.Request) {
	deploymentId := chi.URLParam(r, "deploymentId")
	if deploymentId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing deploymentId", ScaleDeploymentResponse{DeploymentId: "", Replicas: 0})
		return
	}

	var req ScaleDeploymentRequest
	if err := apimodel.DecodeRequestBody(r, &req); err != nil {
		apimodel.RespondFail(w, apimodel.DecodeErrorStatus(err), "invalid json: "+err.Error(), ScaleDeploymentResponse{DeploymentId: deploymentId})
		return
	}
	if req.Replicas < 0 {
		apimodel.RespondFail(w, http.StatusBadRequest, "replicas must be >= 0", ScaleDeploymentResponse{DeploymentId: deploymentId})
		return
	}

	deploy, err := h.psmHandler.GetDeployment(deploymentId)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "get failed: "+err.Error(), ScaleDeploymentResponse{DeploymentId: deploymentId})
		return
	}
	if deploy.Spec.ReplicaSetId != "" {
		if err := h.psmHandler.UpdateReplicaSetReplicas(deploy.Spec.ReplicaSetId, req.Replicas); err != nil {
			apimodel.RespondFail(w, http.StatusInternalServerError, "replicaset scale failed: "+err.Error(), ScaleDeploymentResponse{DeploymentId: deploymentId})
			return
		}
	}
	if err := h.psmHandler.UpdateDeploymentReplicas(deploymentId, req.Replicas); err != nil {
		if deploy.Spec.ReplicaSetId != "" {
			_ = h.psmHandler.UpdateReplicaSetReplicas(deploy.Spec.ReplicaSetId, deploy.Spec.Replicas)
		}
		apimodel.RespondFail(w, http.StatusInternalServerError, "scale failed: "+err.Error(), ScaleDeploymentResponse{DeploymentId: deploymentId})
		return
	}

	apimodel.RespondSuccess(w, http.StatusOK, "deployment scaled", ScaleDeploymentResponse{DeploymentId: deploymentId, Replicas: req.Replicas})
}

// GetDeploymentList godoc
// @Summary list deployments
// @Description list deployments
// @Tags deployments
// @Produce json
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/deployments [get]
func (h *RequestHandler) GetDeploymentList(w http.ResponseWriter, r *http.Request) {
	namespaceFilter := r.URL.Query().Get("namespace")
	list, err := h.psmHandler.GetDeploymentList()
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "list failed: "+err.Error(), nil)
		return
	}
	res := make([]DeploymentSummary, 0, len(list))
	for _, deploy := range list {
		if namespaceFilter != "" && deploy.Spec.Namespace != namespaceFilter {
			continue
		}
		current, ready, _ := h.deploymentReplicaCounts(deploy)
		res = append(res, DeploymentSummary{
			DeploymentId: deploy.DeploymentId,
			Name:         deploy.Spec.Name,
			Namespace:    deploy.Spec.Namespace,
			Replicas:     deploy.Spec.Replicas,
			Desired:      deploy.Spec.Replicas,
			Current:      current,
			Ready:        ready,
			ReplicaSetId: deploy.Spec.ReplicaSetId,
			TemplateId:   deploy.Spec.TemplateId,
			Selector:     deploy.Spec.Selector,
			CreatedAt:    deploy.CreatedAt.Format(time.RFC3339),
			UpdatedAt:    deploy.UpdatedAt.Format(time.RFC3339),
		})
	}
	apimodel.RespondSuccess(w, http.StatusOK, "deployment list", res)
}

// GetDeploymentById godoc
// @Summary get deployment detail
// @Description get deployment detail
// @Tags deployments
// @Param deploymentId path string true "Deployment ID"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/deployments/{deploymentId} [get]
func (h *RequestHandler) GetDeploymentById(w http.ResponseWriter, r *http.Request) {
	deploymentId := chi.URLParam(r, "deploymentId")
	if deploymentId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing deploymentId", nil)
		return
	}
	deploy, err := h.psmHandler.GetDeployment(deploymentId)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "get failed: "+err.Error(), nil)
		return
	}
	template, err := h.psmHandler.GetPodTemplate(deploy.Spec.TemplateId)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "template lookup failed: "+err.Error(), nil)
		return
	}
	current, ready, err := h.deploymentReplicaCounts(deploy)
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "replica lookup failed: "+err.Error(), nil)
		return
	}
	apimodel.RespondSuccess(w, http.StatusOK, "deployment detail", DeploymentDetail{
		DeploymentId: deploy.DeploymentId,
		Name:         deploy.Spec.Name,
		Namespace:    deploy.Spec.Namespace,
		Replicas:     deploy.Spec.Replicas,
		Desired:      deploy.Spec.Replicas,
		Current:      current,
		Ready:        ready,
		ReplicaSetId: deploy.Spec.ReplicaSetId,
		Selector:     deploy.Spec.Selector,
		Template:     template.Spec,
		CreatedAt:    deploy.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    deploy.UpdatedAt.Format(time.RFC3339),
	})
}

// RemoveDeployment godoc
// @Summary remove deployment
// @Description remove deployment
// @Tags deployments
// @Param deploymentId path string true "Deployment ID"
// @Success 200 {object} apimodel.ApiResponse
// @Router /v1/deployments/{deploymentId} [delete]
func (h *RequestHandler) RemoveDeployment(w http.ResponseWriter, r *http.Request) {
	deploymentId := chi.URLParam(r, "deploymentId")
	if deploymentId == "" {
		apimodel.RespondFail(w, http.StatusBadRequest, "missing deploymentId", nil)
		return
	}
	if err := h.removeDeploymentById(deploymentId); err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "remove failed: "+err.Error(), nil)
		return
	}
	apimodel.RespondSuccess(w, http.StatusOK, "deployment removed", map[string]string{"deploymentId": deploymentId})
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
	namespaceFilter := r.URL.Query().Get("namespace")
	podList, err := h.serviceHandler.GetPodList()
	if err != nil {
		apimodel.RespondFail(w, http.StatusInternalServerError, "retrieve pod list failed: "+err.Error(), nil)
		return
	}
	if namespaceFilter != "" {
		filtered := podList[:0]
		for _, p := range podList {
			if p.Namespace == namespaceFilter {
				filtered = append(filtered, p)
			}
		}
		podList = filtered
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

func rootlessFromHostUsers(hostUsers *bool) bool {
	return hostUsers != nil && !*hostUsers
}
