package resource

import (
	"net/http"
	"reflect"

	"raind/internal/condenser/core/container"
	"raind/internal/condenser/core/pod"
	corePVC "raind/internal/condenser/core/pvc"
	"raind/internal/condenser/store/cfm"
	"raind/internal/condenser/store/ism"
	"raind/internal/condenser/store/psm"
	"raind/internal/condenser/store/sec"
	"raind/internal/condenser/store/ssm"
	"raind/internal/condenser/store/vsm"
)

const (
	applyActionCreated   = "created"
	applyActionUpdated   = "updated"
	applyActionUnchanged = "unchanged"
)

func (s *ResourceService) rolloutDeploymentManifest(existing psm.DeploymentInfo, m pod.PodManifest, result *ApplyResult, rollback *[]func()) error {
	currentTemplate, err := s.psmHandler.GetPodTemplate(existing.Spec.TemplateId)
	if err != nil {
		return statusError(http.StatusInternalServerError, "template lookup failed: %v", err)
	}
	desiredTemplateSpec := templateSpecFromManifest(m)
	desiredSpec := deploymentSpecFromManifest(m, existing.Spec.TemplateId, existing.Spec.ReplicaSetId)
	templateUnchanged := podTemplateSpecEqual(currentTemplate.Spec, desiredTemplateSpec)
	if templateUnchanged && deploymentSpecEqual(existing.Spec, desiredSpec) {
		result.Deployments = append(result.Deployments, ApplyDeploymentResult{
			DeploymentId: existing.DeploymentId,
			Namespace:    existing.Spec.Namespace,
			Name:         existing.Spec.Name,
			Replicas:     existing.Spec.Replicas,
			Action:       applyActionUnchanged,
		})
		return nil
	}

	var oldReplicaSet psm.ReplicaSetInfo
	var hasReplicaSet bool
	if existing.Spec.ReplicaSetId != "" {
		if rs, err := s.psmHandler.GetReplicaSet(existing.Spec.ReplicaSetId); err == nil {
			oldReplicaSet = rs
			hasReplicaSet = true
		}
	}

	newTemplateId := existing.Spec.TemplateId
	if !templateUnchanged {
		var err error
		newTemplateId, err = s.storeTemplate(m)
		if err != nil {
			return err
		}
		desiredSpec.TemplateId = newTemplateId
	}
	if err := s.psmHandler.UpdateDeploymentSpec(existing.DeploymentId, desiredSpec); err != nil {
		if !templateUnchanged {
			_ = s.psmHandler.RemovePodTemplate(newTemplateId)
		}
		return statusError(http.StatusInternalServerError, "deployment update failed: %v", err)
	}
	if hasReplicaSet {
		rsSpec := replicaSetSpecFromManifest(m, newTemplateId)
		if err := s.psmHandler.UpdateReplicaSetSpec(oldReplicaSet.ReplicaSetId, rsSpec); err != nil {
			_ = s.psmHandler.UpdateDeploymentSpec(existing.DeploymentId, existing.Spec)
			if !templateUnchanged {
				_ = s.psmHandler.RemovePodTemplate(newTemplateId)
			}
			return statusError(http.StatusInternalServerError, "replicaset update failed: %v", err)
		}
		if !templateUnchanged {
			if err := s.removePodsByReplicaSetId(oldReplicaSet.ReplicaSetId); err != nil {
				return statusError(http.StatusInternalServerError, "rollout pod cleanup failed: %v", err)
			}
		}
	}
	*rollback = append(*rollback, func() {
		_ = s.psmHandler.StorePodTemplate(currentTemplate.TemplateId, currentTemplate.Spec)
		_ = s.psmHandler.UpdateDeploymentSpec(existing.DeploymentId, existing.Spec)
		if hasReplicaSet {
			_ = s.psmHandler.UpdateReplicaSetSpec(oldReplicaSet.ReplicaSetId, oldReplicaSet.Spec)
		}
		if !templateUnchanged {
			_ = s.psmHandler.RemovePodTemplate(newTemplateId)
		}
	})
	if !templateUnchanged && currentTemplate.TemplateId != newTemplateId {
		if inUse, err := s.psmHandler.IsTemplateReferenced(currentTemplate.TemplateId); err == nil && !inUse {
			_ = s.psmHandler.RemovePodTemplate(currentTemplate.TemplateId)
		}
	}
	result.Deployments = append(result.Deployments, ApplyDeploymentResult{
		DeploymentId: existing.DeploymentId,
		Namespace:    m.Namespace,
		Name:         m.Name,
		Replicas:     m.Replicas,
		Action:       applyActionUpdated,
	})
	return nil
}

func (s *ResourceService) rolloutReplicaSetManifest(existing psm.ReplicaSetInfo, m pod.PodManifest, result *ApplyResult, rollback *[]func()) error {
	currentTemplate, err := s.psmHandler.GetPodTemplate(existing.Spec.TemplateId)
	if err != nil {
		return statusError(http.StatusInternalServerError, "template lookup failed: %v", err)
	}
	desiredTemplateSpec := templateSpecFromManifest(m)
	desiredSpec := replicaSetSpecFromManifest(m, existing.Spec.TemplateId)
	templateUnchanged := podTemplateSpecEqual(currentTemplate.Spec, desiredTemplateSpec)
	if templateUnchanged && replicaSetSpecEqual(existing.Spec, desiredSpec) {
		result.ReplicaSets = append(result.ReplicaSets, ApplyReplicaSetResult{
			ReplicaSetId: existing.ReplicaSetId,
			Namespace:    existing.Spec.Namespace,
			Name:         existing.Spec.Name,
			Action:       applyActionUnchanged,
		})
		return nil
	}

	newTemplateId := existing.Spec.TemplateId
	if !templateUnchanged {
		var err error
		newTemplateId, err = s.storeTemplate(m)
		if err != nil {
			return err
		}
		desiredSpec.TemplateId = newTemplateId
	}
	if err := s.psmHandler.UpdateReplicaSetSpec(existing.ReplicaSetId, desiredSpec); err != nil {
		if !templateUnchanged {
			_ = s.psmHandler.RemovePodTemplate(newTemplateId)
		}
		return statusError(http.StatusInternalServerError, "replicaset update failed: %v", err)
	}
	if !templateUnchanged {
		if err := s.removePodsByReplicaSetId(existing.ReplicaSetId); err != nil {
			return statusError(http.StatusInternalServerError, "rollout pod cleanup failed: %v", err)
		}
	}
	*rollback = append(*rollback, func() {
		_ = s.psmHandler.StorePodTemplate(currentTemplate.TemplateId, currentTemplate.Spec)
		_ = s.psmHandler.UpdateReplicaSetSpec(existing.ReplicaSetId, existing.Spec)
		if !templateUnchanged {
			_ = s.psmHandler.RemovePodTemplate(newTemplateId)
		}
	})
	if !templateUnchanged && currentTemplate.TemplateId != newTemplateId {
		if inUse, err := s.psmHandler.IsTemplateReferenced(currentTemplate.TemplateId); err == nil && !inUse {
			_ = s.psmHandler.RemovePodTemplate(currentTemplate.TemplateId)
		}
	}
	result.ReplicaSets = append(result.ReplicaSets, ApplyReplicaSetResult{
		ReplicaSetId: existing.ReplicaSetId,
		Namespace:    m.Namespace,
		Name:         m.Name,
		Action:       applyActionUpdated,
	})
	return nil
}

func (s *ResourceService) rolloutPodManifest(existing psm.PodInfo, m pod.PodManifest, result *ApplyResult, rollback *[]func()) error {
	if existing.TemplateId != "" {
		if currentTemplate, err := s.psmHandler.GetPodTemplate(existing.TemplateId); err == nil && podTemplateSpecEqual(currentTemplate.Spec, templateSpecFromManifest(m)) {
			var ids []string
			if s.containerHandler != nil {
				if containers, err := s.containerHandler.GetContainersByPodId(existing.PodId); err == nil {
					ids = containerIDs(containers)
				}
			}
			result.Pods = append(result.Pods, ApplyPodResult{
				PodId:        existing.PodId,
				Namespace:    existing.Namespace,
				Name:         existing.Name,
				ContainerIds: ids,
				Action:       applyActionUnchanged,
			})
			return nil
		}
	}
	if _, err := s.podHandler.Remove(existing.PodId); err != nil {
		return statusError(http.StatusInternalServerError, "pod remove failed: %v", err)
	}
	*rollback = append(*rollback, func() {
		if existing.TemplateId != "" {
			if podId, err := s.podHandler.CreateFromTemplate(existing.TemplateId, existing.Name); err == nil {
				_, _ = s.podHandler.Start(podId)
			}
		}
	})
	before := len(result.Pods)
	if err := s.applyPodManifest(m, result, rollback); err != nil {
		return err
	}
	for i := before; i < len(result.Pods); i++ {
		result.Pods[i].Action = applyActionUpdated
	}
	return nil
}

func (s *ResourceService) findDeployment(name, namespace string) (psm.DeploymentInfo, bool, error) {
	deployments, err := s.psmHandler.GetDeploymentList()
	if err != nil {
		return psm.DeploymentInfo{}, false, err
	}
	for _, deploy := range deployments {
		if deploy.Spec.Name == name && deploy.Spec.Namespace == namespace {
			return deploy, true, nil
		}
	}
	return psm.DeploymentInfo{}, false, nil
}

func (s *ResourceService) findReplicaSet(name, namespace string) (psm.ReplicaSetInfo, bool, error) {
	replicaSets, err := s.psmHandler.GetReplicaSetList()
	if err != nil {
		return psm.ReplicaSetInfo{}, false, err
	}
	for _, rs := range replicaSets {
		if rs.Spec.Name == name && rs.Spec.Namespace == namespace {
			return rs, true, nil
		}
	}
	return psm.ReplicaSetInfo{}, false, nil
}

func (s *ResourceService) findPod(name, namespace string) (psm.PodInfo, bool, error) {
	pods, err := s.psmHandler.GetPodList()
	if err != nil {
		return psm.PodInfo{}, false, err
	}
	for _, p := range pods {
		if p.Name == name && p.Namespace == namespace {
			return p, true, nil
		}
	}
	return psm.PodInfo{}, false, nil
}

func (s *ResourceService) findService(name, namespace string) (ssm.ServiceInfo, bool, error) {
	services, err := s.ssmHandler.GetServiceList()
	if err != nil {
		return ssm.ServiceInfo{}, false, err
	}
	for _, service := range services {
		if service.Name == name && service.Namespace == namespace {
			return service, true, nil
		}
	}
	return ssm.ServiceInfo{}, false, nil
}

func (s *ResourceService) findIngress(name, namespace string) (ism.IngressInfo, bool, error) {
	ingresses, err := s.ismHandler.GetIngressList()
	if err != nil {
		return ism.IngressInfo{}, false, err
	}
	for _, ingress := range ingresses {
		if ingress.Name == name && ingress.Namespace == namespace {
			return ingress, true, nil
		}
	}
	return ism.IngressInfo{}, false, nil
}

func (s *ResourceService) removePodsByReplicaSetId(replicaSetId string) error {
	pods, err := s.psmHandler.GetPodList()
	if err != nil {
		return err
	}
	for _, p := range pods {
		if p.OwnerKind != psm.OwnerKindReplicaSet || p.OwnerId != replicaSetId {
			continue
		}
		if _, err := s.podHandler.Remove(p.PodId); err != nil {
			return err
		}
	}
	return nil
}

func templateSpecFromManifest(m pod.PodManifest) psm.PodTemplateSpec {
	return psm.PodTemplateSpec{
		Name:        m.Name,
		Namespace:   m.Namespace,
		Labels:      m.Labels,
		Annotations: m.Annotations,
		Rootless:    m.Rootless,
		Containers:  m.Containers,
	}
}

func replicaSetSpecFromManifest(m pod.PodManifest, templateId string) psm.ReplicaSetSpec {
	return psm.ReplicaSetSpec{
		Name:       m.Name,
		Namespace:  m.Namespace,
		Replicas:   m.Replicas,
		TemplateId: templateId,
		Selector:   m.Selector,
	}
}

func deploymentSpecFromManifest(m pod.PodManifest, templateId, replicaSetId string) psm.DeploymentSpec {
	return psm.DeploymentSpec{
		Name:         m.Name,
		Namespace:    m.Namespace,
		Replicas:     m.Replicas,
		TemplateId:   templateId,
		Selector:     m.Selector,
		ReplicaSetId: replicaSetId,
	}
}

func podTemplateSpecEqual(a, b psm.PodTemplateSpec) bool {
	return reflect.DeepEqual(normalizeTemplateSpec(a), normalizeTemplateSpec(b))
}

func replicaSetSpecEqual(a, b psm.ReplicaSetSpec) bool {
	return reflect.DeepEqual(normalizeReplicaSetSpec(a), normalizeReplicaSetSpec(b))
}

func deploymentSpecEqual(a, b psm.DeploymentSpec) bool {
	return reflect.DeepEqual(normalizeDeploymentSpec(a), normalizeDeploymentSpec(b))
}

func normalizeTemplateSpec(spec psm.PodTemplateSpec) psm.PodTemplateSpec {
	spec.Labels = nilIfEmptyMap(spec.Labels)
	spec.Annotations = nilIfEmptyMap(spec.Annotations)
	for i := range spec.Containers {
		spec.Containers[i].Command = nilIfEmptySlice(spec.Containers[i].Command)
		spec.Containers[i].Port = nilIfEmptySlice(spec.Containers[i].Port)
		spec.Containers[i].Mount = nilIfEmptySlice(spec.Containers[i].Mount)
		spec.Containers[i].Env = nilIfEmptySlice(spec.Containers[i].Env)
		spec.Containers[i].CapAdd = nilIfEmptySlice(spec.Containers[i].CapAdd)
		spec.Containers[i].CapDrop = nilIfEmptySlice(spec.Containers[i].CapDrop)
		if spec.Containers[i].SecurityProfile == "default" {
			spec.Containers[i].SecurityProfile = ""
		}
	}
	return spec
}

func normalizeReplicaSetSpec(spec psm.ReplicaSetSpec) psm.ReplicaSetSpec {
	spec.Selector = nilIfEmptyMap(spec.Selector)
	return spec
}

func normalizeDeploymentSpec(spec psm.DeploymentSpec) psm.DeploymentSpec {
	spec.Selector = nilIfEmptyMap(spec.Selector)
	return spec
}

func nilIfEmptySlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	return in
}

func nilIfEmptyMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	return in
}

func defaultNamespace(namespace string) string {
	if namespace == "" {
		return "default"
	}
	return namespace
}

func stringMapEqual(a, b map[string]string) bool {
	return reflect.DeepEqual(nilIfEmptyMap(a), nilIfEmptyMap(b))
}

func configMapInfoEqual(existing cfm.ConfigMapInfo, desired cfm.ConfigMapInfo) bool {
	return existing.Name == desired.Name &&
		defaultNamespace(existing.Namespace) == defaultNamespace(desired.Namespace) &&
		stringMapEqual(existing.Data, desired.Data)
}

func secretInfoEqual(existing sec.SecretInfo, desired sec.SecretInfo) bool {
	existingType := existing.Type
	if existingType == "" {
		existingType = sec.SecretTypeOpaque
	}
	desiredType := desired.Type
	if desiredType == "" {
		desiredType = sec.SecretTypeOpaque
	}
	return existing.Name == desired.Name &&
		defaultNamespace(existing.Namespace) == defaultNamespace(desired.Namespace) &&
		existingType == desiredType &&
		stringMapEqual(existing.Data, desired.Data)
}

func serviceInfoEqual(existing ssm.ServiceInfo, desired ssm.ServiceInfo) bool {
	return existing.Name == desired.Name &&
		existing.Namespace == desired.Namespace &&
		ssm.NormalizeServiceType(existing.Type) == ssm.NormalizeServiceType(desired.Type) &&
		existing.ClusterIP == desired.ClusterIP &&
		stringMapEqual(existing.Selector, desired.Selector) &&
		reflect.DeepEqual(existing.Ports, desired.Ports)
}

func ingressInfoEqual(existing ism.IngressInfo, desired ism.IngressInfo) bool {
	return existing.Name == desired.Name &&
		existing.Namespace == desired.Namespace &&
		reflect.DeepEqual(existing.Rules, desired.Rules) &&
		reflect.DeepEqual(nilIfEmptySlice(existing.TLSHosts), nilIfEmptySlice(desired.TLSHosts))
}

func pvcManifestEqual(existing vsm.PersistentVolumeClaimInfo, manifest corePVC.Manifest) bool {
	return existing.Name == manifest.Name &&
		defaultNamespace(existing.Namespace) == defaultNamespace(manifest.Namespace) &&
		reflect.DeepEqual(existing.AccessModes, manifest.AccessModes) &&
		existing.RequestedStorage == manifest.RequestedStorage &&
		existing.RequestedBytes == manifest.RequestedBytes &&
		existing.StorageClassName == manifest.StorageClassName &&
		existing.VolumeMode == manifest.VolumeMode &&
		existing.ReclaimPolicy == manifest.ReclaimPolicy
}

func containerIDs(containers []container.ContainerState) []string {
	ids := make([]string, 0, len(containers))
	for _, c := range containers {
		ids = append(ids, c.ContainerId)
	}
	return ids
}
