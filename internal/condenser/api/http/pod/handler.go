package pod

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	apimodel "raind/internal/condenser/api/http/utils"
	"raind/internal/condenser/core/container"
	coreIngress "raind/internal/condenser/core/ingress"
	corenamespace "raind/internal/condenser/core/namespace"
	"raind/internal/condenser/core/pod"
	coreService "raind/internal/condenser/core/service"
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
	body, err := io.ReadAll(r.Body)
	if err != nil {
		apimodel.RespondFail(w, http.StatusBadRequest, "invalid body: "+err.Error(), nil)
		return
	}

	var results []ApplyPodResult
	var replicaSetResults []ApplyReplicaSetResult
	var deploymentResults []ApplyDeploymentResult
	var serviceResults []ApplyServiceResult
	var ingressResults []ApplyIngressResult
	var namespaceResults []ApplyNamespaceResult
	var rollback []func()
	rollbackApplied := func() {
		for i := len(rollback) - 1; i >= 0; i-- {
			rollback[i]()
		}
	}

	dec := yaml.NewDecoder(bytes.NewReader(body))
	for {
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			rollbackApplied()
			apimodel.RespondFail(w, http.StatusBadRequest, invalidYAMLErrorMessage(err), nil)
			return
		}
		if len(raw) == 0 {
			continue
		}
		kind, _ := raw["kind"].(string)
		if kind == "" {
			rollbackApplied()
			apimodel.RespondFail(w, http.StatusBadRequest, "kind is required", nil)
			return
		}
		rawBytes, err := yaml.Marshal(raw)
		if err != nil {
			rollbackApplied()
			apimodel.RespondFail(w, http.StatusBadRequest, invalidYAMLErrorMessage(err), nil)
			return
		}

		switch kind {
		case "Namespace":
			manifest, err := decodeNamespaceManifest(rawBytes)
			if err != nil {
				rollbackApplied()
				apimodel.RespondFail(w, http.StatusBadRequest, invalidYAMLErrorMessage(err), nil)
				return
			}
			info, err := h.namespaceHandler.Create(corenamespace.ServiceCreateModel{
				Name:        manifest.Metadata.Name,
				Labels:      manifest.Metadata.Labels,
				Annotations: manifest.Metadata.Annotations,
			})
			if err != nil {
				rollbackApplied()
				apimodel.RespondFail(w, http.StatusInternalServerError, "namespace create failed: "+err.Error(), nil)
				return
			}
			rollback = append(rollback, func() {
				_, _ = h.namespaceHandler.Remove(corenamespace.ServiceRemoveModel{Name: info.Name})
			})
			namespaceResults = append(namespaceResults, ApplyNamespaceResult{Name: info.Name, Network: info.Network})

		case "Service":
			manifest, err := coreService.DecodeK8sServiceManifest(rawBytes)
			if err != nil {
				rollbackApplied()
				apimodel.RespondFail(w, http.StatusBadRequest, invalidYAMLErrorMessage(err), nil)
				return
			}
			if manifest.Name == "" || manifest.Namespace == "" {
				rollbackApplied()
				apimodel.RespondFail(w, http.StatusBadRequest, "name and namespace are required", nil)
				return
			}
			if h.ssmHandler.IsNameAlreadyUsed(manifest.Name, manifest.Namespace) {
				rollbackApplied()
				apimodel.RespondFail(w, http.StatusBadRequest, "name already used by other service", nil)
				return
			}
			serviceId := utils.NewUlid()
			if err := h.ssmHandler.StoreService(serviceId, ssm.ServiceInfo{
				Name:      manifest.Name,
				Namespace: manifest.Namespace,
				Type:      manifest.Type,
				ClusterIP: manifest.ClusterIP,
				Selector:  manifest.Selector,
				Ports:     manifest.Ports,
			}); err != nil {
				rollbackApplied()
				apimodel.RespondFail(w, http.StatusInternalServerError, "service store failed: "+err.Error(), nil)
				return
			}
			rollback = append(rollback, func() {
				_ = h.ssmHandler.RemoveService(serviceId)
			})
			serviceResults = append(serviceResults, ApplyServiceResult{
				ServiceId: serviceId,
				Name:      manifest.Name,
				Namespace: manifest.Namespace,
			})

		case "Ingress":
			manifest, err := coreIngress.DecodeK8sIngressManifest(rawBytes)
			if err != nil {
				rollbackApplied()
				apimodel.RespondFail(w, http.StatusBadRequest, invalidYAMLErrorMessage(err), nil)
				return
			}
			if h.ismHandler.IsNameAlreadyUsed(manifest.Name, manifest.Namespace) {
				rollbackApplied()
				apimodel.RespondFail(w, http.StatusBadRequest, "name already used by other ingress", nil)
				return
			}
			ingressId := utils.NewUlid()
			if err := h.ismHandler.StoreIngress(ingressId, ism.IngressInfo{
				Name:      manifest.Name,
				Namespace: manifest.Namespace,
				Rules:     manifest.Rules,
			}); err != nil {
				rollbackApplied()
				apimodel.RespondFail(w, http.StatusInternalServerError, "ingress store failed: "+err.Error(), nil)
				return
			}
			rollback = append(rollback, func() {
				_ = h.ismHandler.RemoveIngress(ingressId)
			})
			ingressResults = append(ingressResults, ApplyIngressResult{
				IngressId: ingressId,
				Name:      manifest.Name,
				Namespace: manifest.Namespace,
			})

		case "Pod", "ReplicaSet", "Deployment":
			manifests, err := pod.DecodeK8sManifests(rawBytes)
			if err != nil || len(manifests) == 0 {
				rollbackApplied()
				apimodel.RespondFail(w, http.StatusBadRequest, invalidYAMLErrorMessage(err), nil)
				return
			}
			m := manifests[0]
			if m.Kind == "Deployment" {
				if err := h.resolveManifestNetworks(&m); err != nil {
					rollbackApplied()
					apimodel.RespondFail(w, http.StatusBadRequest, err.Error(), nil)
					return
				}
				if err := h.ensureResourceNameAvailable(m.Name, m.Namespace); err != nil {
					rollbackApplied()
					apimodel.RespondFail(w, http.StatusBadRequest, err.Error(), nil)
					return
				}
				templateId := utils.NewUlid()
				if err := h.psmHandler.StorePodTemplate(templateId, psm.PodTemplateSpec{
					Name:        m.Name,
					Namespace:   m.Namespace,
					Labels:      m.Labels,
					Annotations: m.Annotations,
					Containers:  m.Containers,
				}); err != nil {
					rollbackApplied()
					apimodel.RespondFail(w, http.StatusInternalServerError, "template store failed: "+err.Error(), nil)
					return
				}
				rollback = append(rollback, func() {
					_ = h.psmHandler.RemovePodTemplate(templateId)
				})
				deploymentId := utils.NewUlid()
				if err := h.psmHandler.StoreDeployment(deploymentId, psm.DeploymentSpec{
					Name:       m.Name,
					Namespace:  m.Namespace,
					Replicas:   m.Replicas,
					TemplateId: templateId,
					Selector:   m.Selector,
				}); err != nil {
					rollbackApplied()
					apimodel.RespondFail(w, http.StatusInternalServerError, "deployment store failed: "+err.Error(), nil)
					return
				}
				rollback = append(rollback, func() {
					_ = h.removeDeploymentById(deploymentId)
				})
				deploymentResults = append(deploymentResults, ApplyDeploymentResult{
					DeploymentId: deploymentId,
					Namespace:    m.Namespace,
					Name:         m.Name,
					Replicas:     m.Replicas,
				})
				continue
			}
			if m.Kind == "ReplicaSet" {
				if err := h.resolveManifestNetworks(&m); err != nil {
					rollbackApplied()
					apimodel.RespondFail(w, http.StatusBadRequest, err.Error(), nil)
					return
				}
				if err := h.ensureResourceNameAvailable(m.Name, m.Namespace); err != nil {
					rollbackApplied()
					apimodel.RespondFail(w, http.StatusBadRequest, err.Error(), nil)
					return
				}
				templateId := utils.NewUlid()
				if err := h.psmHandler.StorePodTemplate(templateId, psm.PodTemplateSpec{
					Name:        m.Name,
					Namespace:   m.Namespace,
					Labels:      m.Labels,
					Annotations: m.Annotations,
					Containers:  m.Containers,
				}); err != nil {
					rollbackApplied()
					apimodel.RespondFail(w, http.StatusInternalServerError, "template store failed: "+err.Error(), nil)
					return
				}
				rollback = append(rollback, func() {
					_ = h.psmHandler.RemovePodTemplate(templateId)
				})
				replicaSetId := utils.NewUlid()
				if err := h.psmHandler.StoreReplicaSet(replicaSetId, psm.ReplicaSetSpec{
					Name:       m.Name,
					Namespace:  m.Namespace,
					Replicas:   m.Replicas,
					TemplateId: templateId,
					Selector:   m.Selector,
				}); err != nil {
					rollbackApplied()
					apimodel.RespondFail(w, http.StatusInternalServerError, "replicaset store failed: "+err.Error(), nil)
					return
				}
				rollback = append(rollback, func() {
					_ = h.removeReplicaSetById(replicaSetId)
				})
				replicaSetResults = append(replicaSetResults, ApplyReplicaSetResult{
					ReplicaSetId: replicaSetId,
					Namespace:    m.Namespace,
					Name:         m.Name,
				})
				continue
			}

			if err := h.resolveManifestNetworks(&m); err != nil {
				rollbackApplied()
				apimodel.RespondFail(w, http.StatusBadRequest, err.Error(), nil)
				return
			}
			podId, err := h.serviceHandler.Create(pod.ServiceCreateModel{
				Name:        m.Name,
				Namespace:   m.Namespace,
				Labels:      m.Labels,
				Annotations: m.Annotations,
				Containers:  m.Containers,
			})
			if err != nil {
				rollbackApplied()
				apimodel.RespondFail(w, http.StatusInternalServerError, "pod create failed: "+err.Error(), nil)
				return
			}
			rollback = append(rollback, func() {
				_, _ = h.serviceHandler.Remove(podId)
			})

			if _, err := h.serviceHandler.Start(podId); err != nil {
				_, _ = h.serviceHandler.Remove(podId)
				rollbackApplied()
				apimodel.RespondFail(w, http.StatusInternalServerError, "pod start failed: "+err.Error(), nil)
				return
			}

			containers, err := h.containerHandler.GetContainersByPodId(podId)
			if err != nil {
				_, _ = h.serviceHandler.Remove(podId)
				rollbackApplied()
				apimodel.RespondFail(w, http.StatusInternalServerError, "container list failed: "+err.Error(), nil)
				return
			}
			containerIds := make([]string, 0, len(containers))
			for _, c := range containers {
				if strings.HasPrefix(c.Name, utils.PodInfraContainerNamePrefix) {
					continue
				}
				containerIds = append(containerIds, c.ContainerId)
			}

			results = append(results, ApplyPodResult{
				PodId:        podId,
				Namespace:    m.Namespace,
				Name:         m.Name,
				ContainerIds: containerIds,
			})

		default:
			rollbackApplied()
			apimodel.RespondFail(w, http.StatusBadRequest, "unsupported kind: "+kind, nil)
			return
		}
	}

	apimodel.RespondSuccess(w, http.StatusCreated, "resources applied", ApplyPodResponse{
		Pods:        results,
		ReplicaSets: replicaSetResults,
		Deployments: deploymentResults,
		Services:    serviceResults,
		Ingresses:   ingressResults,
		Namespaces:  namespaceResults,
	})
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
	var deployResults []DeleteDeploymentResult
	var svcResults []DeleteServiceResult
	var ingressResults []DeleteIngressResult
	var namespaceResults []DeleteNamespaceResult
	var pendingNamespaceDeletes []string

	dec := yaml.NewDecoder(bytes.NewReader(body))
	for {
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			apimodel.RespondFail(w, http.StatusBadRequest, invalidYAMLErrorMessage(err), nil)
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
			apimodel.RespondFail(w, http.StatusBadRequest, invalidYAMLErrorMessage(err), nil)
			return
		}

		switch kind {
		case "Namespace":
			manifest, err := decodeNamespaceManifest(rawBytes)
			if err != nil {
				apimodel.RespondFail(w, http.StatusBadRequest, invalidYAMLErrorMessage(err), nil)
				return
			}
			pendingNamespaceDeletes = append(pendingNamespaceDeletes, manifest.Metadata.Name)
		case "Service":
			manifest, err := coreService.DecodeK8sServiceManifest(rawBytes)
			if err != nil {
				apimodel.RespondFail(w, http.StatusBadRequest, invalidYAMLErrorMessage(err), nil)
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

		case "Ingress":
			manifest, err := coreIngress.DecodeK8sIngressManifest(rawBytes)
			if err != nil {
				apimodel.RespondFail(w, http.StatusBadRequest, invalidYAMLErrorMessage(err), nil)
				return
			}
			list, err := h.ismHandler.GetIngressList()
			if err != nil {
				apimodel.RespondFail(w, http.StatusInternalServerError, "list failed: "+err.Error(), nil)
				return
			}
			var removed bool
			for _, in := range list {
				if in.Name != manifest.Name || in.Namespace != manifest.Namespace {
					continue
				}
				if err := h.ismHandler.RemoveIngress(in.IngressId); err != nil {
					apimodel.RespondFail(w, http.StatusInternalServerError, "remove failed: "+err.Error(), nil)
					return
				}
				ingressResults = append(ingressResults, DeleteIngressResult{
					IngressId: in.IngressId,
					Name:      in.Name,
					Namespace: in.Namespace,
				})
				removed = true
			}
			if !removed {
				apimodel.RespondFail(w, http.StatusNotFound, "ingress not found", nil)
				return
			}

		case "Deployment":
			manifests, err := pod.DecodeK8sManifests(rawBytes)
			if err != nil || len(manifests) == 0 {
				apimodel.RespondFail(w, http.StatusBadRequest, invalidYAMLErrorMessage(err), nil)
				return
			}
			m := manifests[0]
			list, err := h.psmHandler.GetDeploymentList()
			if err != nil {
				apimodel.RespondFail(w, http.StatusInternalServerError, "list failed: "+err.Error(), nil)
				return
			}
			var removed bool
			for _, deploy := range list {
				if deploy.Spec.Name != m.Name || deploy.Spec.Namespace != m.Namespace {
					continue
				}
				if err := h.removeDeploymentById(deploy.DeploymentId); err != nil {
					apimodel.RespondFail(w, http.StatusInternalServerError, "remove failed: "+err.Error(), nil)
					return
				}
				deployResults = append(deployResults, DeleteDeploymentResult{
					DeploymentId: deploy.DeploymentId,
					Name:         deploy.Spec.Name,
					Namespace:    deploy.Spec.Namespace,
				})
				removed = true
			}
			if !removed {
				apimodel.RespondFail(w, http.StatusNotFound, "deployment not found", nil)
				return
			}

		case "ReplicaSet":
			manifests, err := pod.DecodeK8sManifests(rawBytes)
			if err != nil || len(manifests) == 0 {
				apimodel.RespondFail(w, http.StatusBadRequest, invalidYAMLErrorMessage(err), nil)
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
				apimodel.RespondFail(w, http.StatusBadRequest, invalidYAMLErrorMessage(err), nil)
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

	for _, name := range pendingNamespaceDeletes {
		if _, err := h.namespaceHandler.Remove(corenamespace.ServiceRemoveModel{Name: name}); err != nil {
			apimodel.RespondFail(w, http.StatusInternalServerError, "namespace remove failed: "+err.Error(), nil)
			return
		}
		namespaceResults = append(namespaceResults, DeleteNamespaceResult{Name: name})
	}

	apimodel.RespondSuccess(w, http.StatusOK, "resources deleted", DeleteResourcesResponse{
		Pods:        podResults,
		ReplicaSets: rsResults,
		Deployments: deployResults,
		Services:    svcResults,
		Ingresses:   ingressResults,
		Namespaces:  namespaceResults,
	})
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
			if p.TemplateId != rs.Spec.TemplateId {
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
		if p.TemplateId != rs.Spec.TemplateId {
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
		if p.TemplateId != rs.Spec.TemplateId {
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
