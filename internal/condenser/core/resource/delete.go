package resource

import (
	"bytes"
	"io"
	"net/http"
	"sort"
	"strings"

	coreConfigMap "raind/internal/condenser/core/configmap"
	coreIngress "raind/internal/condenser/core/ingress"
	corenamespace "raind/internal/condenser/core/namespace"
	coreNetworkPolicy "raind/internal/condenser/core/networkpolicy"
	"raind/internal/condenser/core/pod"
	corePVC "raind/internal/condenser/core/pvc"
	coreSecret "raind/internal/condenser/core/secret"
	coreService "raind/internal/condenser/core/service"

	"gopkg.in/yaml.v3"
)

type deleteManifestDocument struct {
	RawBytes []byte
	Header   Header
}

func (s *ResourceService) Delete(body []byte) (DeleteResult, error) {
	var result DeleteResult

	docs, warnings, err := decodeDeleteManifestDocuments(body)
	if err != nil {
		return DeleteResult{}, err
	}
	result.Warnings = append(result.Warnings, warnings...)

	for _, doc := range sortDeleteManifestDocuments(docs) {
		if err := s.deleteManifestDocument(doc, &result); err != nil {
			return DeleteResult{}, err
		}
	}

	return result, nil
}

func decodeDeleteManifestDocuments(body []byte) ([]deleteManifestDocument, []Warning, error) {
	var docs []deleteManifestDocument
	var warnings []Warning

	dec := yaml.NewDecoder(bytes.NewReader(body))
	for {
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			return nil, nil, statusMessage(http.StatusBadRequest, invalidYAMLErrorMessage(err))
		}
		if len(raw) == 0 {
			continue
		}
		rawBytes, err := yaml.Marshal(raw)
		if err != nil {
			return nil, nil, statusMessage(http.StatusBadRequest, invalidYAMLErrorMessage(err))
		}
		header, err := decodeHeader(rawBytes)
		if err != nil {
			return nil, nil, statusMessage(http.StatusBadRequest, err.Error())
		}
		warnings = append(warnings, collectHeaderWarnings(header, raw)...)
		docs = append(docs, deleteManifestDocument{RawBytes: rawBytes, Header: header})
	}

	return docs, warnings, nil
}

func sortDeleteManifestDocuments(docs []deleteManifestDocument) []deleteManifestDocument {
	ordered := append([]deleteManifestDocument(nil), docs...)
	stableSortDeleteDocuments(ordered)
	return ordered
}

func stableSortDeleteDocuments(docs []deleteManifestDocument) {
	sort.SliceStable(docs, func(i, j int) bool {
		return deleteKindPriority(docs[i].Header.Kind) < deleteKindPriority(docs[j].Header.Kind)
	})
}

func deleteKindPriority(kind string) int {
	switch kind {
	case "Deployment":
		return 10
	case "ReplicaSet":
		return 20
	case "Pod":
		return 30
	case "Ingress":
		return 40
	case "NetworkPolicy":
		return 50
	case "Service":
		return 60
	case "ConfigMap":
		return 70
	case "Secret":
		return 80
	case "PersistentVolumeClaim":
		return 90
	case "Namespace":
		return 1000
	default:
		return 500
	}
}

func (s *ResourceService) deleteManifestDocument(doc deleteManifestDocument, result *DeleteResult) error {
	rawBytes := doc.RawBytes
	header := doc.Header

	switch header.Kind {
	case "Namespace":
		manifest, err := decodeNamespaceManifest(rawBytes)
		if err != nil {
			return statusMessage(http.StatusBadRequest, invalidYAMLErrorMessage(err))
		}
		if _, err := s.namespaceHandler.Remove(corenamespace.ServiceRemoveModel{Name: manifest.Metadata.Name}); err != nil {
			if appendDeleteNotFoundWarning(result, header, err) {
				return nil
			}
			return statusError(http.StatusInternalServerError, "namespace remove failed: %v", err)
		}
		result.Namespaces = append(result.Namespaces, DeleteNamespaceResult{Name: manifest.Metadata.Name})

	case "Service":
		serviceResults, err := s.deleteService(rawBytes)
		if err != nil {
			if appendDeleteNotFoundWarning(result, header, err) {
				return nil
			}
			return err
		}
		result.Services = append(result.Services, serviceResults...)

	case "Ingress":
		ingressResults, err := s.deleteIngress(rawBytes)
		if err != nil {
			if appendDeleteNotFoundWarning(result, header, err) {
				return nil
			}
			return err
		}
		result.Ingresses = append(result.Ingresses, ingressResults...)

	case "ConfigMap":
		configMapResults, err := s.deleteConfigMap(rawBytes)
		if err != nil {
			if appendDeleteNotFoundWarning(result, header, err) {
				return nil
			}
			return err
		}
		result.ConfigMaps = append(result.ConfigMaps, configMapResults...)

	case "Secret":
		secretResults, err := s.deleteSecret(rawBytes)
		if err != nil {
			if appendDeleteNotFoundWarning(result, header, err) {
				return nil
			}
			return err
		}
		result.Secrets = append(result.Secrets, secretResults...)

	case "NetworkPolicy":
		networkPolicyResults, err := s.deleteNetworkPolicy(rawBytes)
		if err != nil {
			if appendDeleteNotFoundWarning(result, header, err) {
				return nil
			}
			return err
		}
		result.NetworkPolicies = append(result.NetworkPolicies, networkPolicyResults...)

	case "PersistentVolumeClaim":
		pvcResults, err := s.deletePVC(rawBytes)
		if err != nil {
			if appendDeleteNotFoundWarning(result, header, err) {
				return nil
			}
			return err
		}
		result.PersistentVolumeClaims = append(result.PersistentVolumeClaims, pvcResults...)

	case "Deployment":
		deployResults, err := s.deleteDeployment(rawBytes)
		if err != nil {
			if appendDeleteNotFoundWarning(result, header, err) {
				return nil
			}
			return err
		}
		result.Deployments = append(result.Deployments, deployResults...)

	case "ReplicaSet":
		rsResults, err := s.deleteReplicaSet(rawBytes)
		if err != nil {
			if appendDeleteNotFoundWarning(result, header, err) {
				return nil
			}
			return err
		}
		result.ReplicaSets = append(result.ReplicaSets, rsResults...)

	case "Pod":
		podResults, err := s.deletePod(rawBytes)
		if err != nil {
			if appendDeleteNotFoundWarning(result, header, err) {
				return nil
			}
			return err
		}
		result.Pods = append(result.Pods, podResults...)

	default:
		return statusError(http.StatusBadRequest, "unsupported kind: %s", header.Kind)
	}

	return nil
}

func appendDeleteNotFoundWarning(result *DeleteResult, header Header, err error) bool {
	if ErrorStatus(err, http.StatusInternalServerError) != http.StatusNotFound && !strings.Contains(err.Error(), "not found") {
		return false
	}
	result.Warnings = append(result.Warnings, Warning{
		Kind:      header.Kind,
		Name:      header.Metadata.Name,
		Namespace: header.Metadata.Namespace,
		Message:   "resource not found; skipped delete",
	})
	return true
}

func (s *ResourceService) deleteSecret(rawBytes []byte) ([]DeleteSecretResult, error) {
	manifest, err := coreSecret.DecodeK8sSecretManifest(rawBytes)
	if err != nil {
		return nil, statusMessage(http.StatusBadRequest, invalidYAMLErrorMessage(err))
	}
	list, err := s.secHandler.GetSecretList()
	if err != nil {
		return nil, statusError(http.StatusInternalServerError, "list failed: %v", err)
	}
	var result []DeleteSecretResult
	for _, secret := range list {
		if secret.Name != manifest.Name || secret.Namespace != manifest.Namespace {
			continue
		}
		if err := s.secHandler.RemoveSecret(secret.SecretId); err != nil {
			return nil, statusError(http.StatusInternalServerError, "remove failed: %v", err)
		}
		result = append(result, DeleteSecretResult{
			SecretId:  secret.SecretId,
			Name:      secret.Name,
			Namespace: secret.Namespace,
		})
	}
	if len(result) == 0 {
		return nil, statusError(http.StatusNotFound, "secret not found")
	}
	return result, nil
}

func (s *ResourceService) deleteNetworkPolicy(rawBytes []byte) ([]DeleteNetworkPolicyResult, error) {
	manifest, err := coreNetworkPolicy.DecodeK8sNetworkPolicyManifest(rawBytes)
	if err != nil {
		return nil, statusMessage(http.StatusBadRequest, invalidYAMLErrorMessage(err))
	}
	info, err := coreNetworkPolicy.NewService().Remove(manifest.Name, manifest.Namespace)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, statusError(http.StatusNotFound, "networkpolicy not found")
		}
		return nil, statusError(http.StatusInternalServerError, "networkpolicy remove failed: %v", err)
	}
	return []DeleteNetworkPolicyResult{{
		NetworkPolicyId: info.NetworkPolicyId,
		Name:            info.Name,
		Namespace:       info.Namespace,
	}}, nil
}

func (s *ResourceService) deletePVC(rawBytes []byte) ([]DeletePVCResult, error) {
	manifest, err := corePVC.DecodeK8sPVCManifest(rawBytes)
	if err != nil {
		return nil, statusMessage(http.StatusBadRequest, invalidYAMLErrorMessage(err))
	}
	info, err := corePVC.NewService().Remove(manifest.Name, manifest.Namespace)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, statusError(http.StatusNotFound, "persistentvolumeclaim not found")
		}
		return nil, statusError(http.StatusInternalServerError, "persistentvolumeclaim remove failed: %v", err)
	}
	return []DeletePVCResult{{
		PVCId:         info.PVCId,
		Name:          info.Name,
		Namespace:     info.Namespace,
		ReclaimPolicy: info.ReclaimPolicy,
	}}, nil
}

func (s *ResourceService) deleteConfigMap(rawBytes []byte) ([]DeleteConfigMapResult, error) {
	manifest, err := coreConfigMap.DecodeK8sConfigMapManifest(rawBytes)
	if err != nil {
		return nil, statusMessage(http.StatusBadRequest, invalidYAMLErrorMessage(err))
	}
	list, err := s.cfmHandler.GetConfigMapList()
	if err != nil {
		return nil, statusError(http.StatusInternalServerError, "list failed: %v", err)
	}
	var result []DeleteConfigMapResult
	for _, cm := range list {
		if cm.Name != manifest.Name || cm.Namespace != manifest.Namespace {
			continue
		}
		if err := s.cfmHandler.RemoveConfigMap(cm.ConfigMapId); err != nil {
			return nil, statusError(http.StatusInternalServerError, "remove failed: %v", err)
		}
		result = append(result, DeleteConfigMapResult{
			ConfigMapId: cm.ConfigMapId,
			Name:        cm.Name,
			Namespace:   cm.Namespace,
		})
	}
	if len(result) == 0 {
		return nil, statusError(http.StatusNotFound, "configmap not found")
	}
	return result, nil
}

func (s *ResourceService) deleteService(rawBytes []byte) ([]DeleteServiceResult, error) {
	manifest, err := coreService.DecodeK8sServiceManifest(rawBytes)
	if err != nil {
		return nil, statusMessage(http.StatusBadRequest, invalidYAMLErrorMessage(err))
	}
	list, err := s.ssmHandler.GetServiceList()
	if err != nil {
		return nil, statusError(http.StatusInternalServerError, "list failed: %v", err)
	}
	var result []DeleteServiceResult
	for _, svc := range list {
		if svc.Name != manifest.Name || svc.Namespace != manifest.Namespace {
			continue
		}
		if err := s.ssmHandler.RemoveService(svc.ServiceId); err != nil {
			return nil, statusError(http.StatusInternalServerError, "remove failed: %v", err)
		}
		result = append(result, DeleteServiceResult{
			ServiceId: svc.ServiceId,
			Name:      svc.Name,
			Namespace: svc.Namespace,
		})
	}
	if len(result) == 0 {
		return nil, statusError(http.StatusNotFound, "service not found")
	}
	return result, nil
}

func (s *ResourceService) deleteIngress(rawBytes []byte) ([]DeleteIngressResult, error) {
	manifest, err := coreIngress.DecodeK8sIngressManifest(rawBytes)
	if err != nil {
		return nil, statusMessage(http.StatusBadRequest, invalidYAMLErrorMessage(err))
	}
	list, err := s.ismHandler.GetIngressList()
	if err != nil {
		return nil, statusError(http.StatusInternalServerError, "list failed: %v", err)
	}
	var result []DeleteIngressResult
	for _, in := range list {
		if in.Name != manifest.Name || in.Namespace != manifest.Namespace {
			continue
		}
		if err := s.ismHandler.RemoveIngress(in.IngressId); err != nil {
			return nil, statusError(http.StatusInternalServerError, "remove failed: %v", err)
		}
		if err := coreIngress.NewTLSManager().RemoveHostsIfUnused(in.TLSHosts, activeTLSHostsExcept(list, in.IngressId)); err != nil {
			return nil, statusError(http.StatusInternalServerError, "ingress tls certificate cleanup failed: %v", err)
		}
		result = append(result, DeleteIngressResult{
			IngressId: in.IngressId,
			Name:      in.Name,
			Namespace: in.Namespace,
		})
	}
	if len(result) == 0 {
		return nil, statusError(http.StatusNotFound, "ingress not found")
	}
	return result, nil
}

func (s *ResourceService) deleteDeployment(rawBytes []byte) ([]DeleteDeploymentResult, error) {
	manifest, err := decodeSinglePodManifest(rawBytes)
	if err != nil {
		return nil, err
	}
	list, err := s.psmHandler.GetDeploymentList()
	if err != nil {
		return nil, statusError(http.StatusInternalServerError, "list failed: %v", err)
	}
	var result []DeleteDeploymentResult
	for _, deploy := range list {
		if deploy.Spec.Name != manifest.Name || deploy.Spec.Namespace != manifest.Namespace {
			continue
		}
		if err := s.RemoveDeploymentById(deploy.DeploymentId); err != nil {
			return nil, statusError(http.StatusInternalServerError, "remove failed: %v", err)
		}
		result = append(result, DeleteDeploymentResult{
			DeploymentId: deploy.DeploymentId,
			Name:         deploy.Spec.Name,
			Namespace:    deploy.Spec.Namespace,
		})
	}
	if len(result) == 0 {
		return nil, statusError(http.StatusNotFound, "deployment not found")
	}
	return result, nil
}

func (s *ResourceService) deleteReplicaSet(rawBytes []byte) ([]DeleteReplicaSetResult, error) {
	manifest, err := decodeSinglePodManifest(rawBytes)
	if err != nil {
		return nil, err
	}
	list, err := s.psmHandler.GetReplicaSetList()
	if err != nil {
		return nil, statusError(http.StatusInternalServerError, "list failed: %v", err)
	}
	var result []DeleteReplicaSetResult
	for _, rs := range list {
		if rs.Spec.Name != manifest.Name || rs.Spec.Namespace != manifest.Namespace {
			continue
		}
		if err := s.RemoveReplicaSetById(rs.ReplicaSetId); err != nil {
			return nil, statusError(http.StatusInternalServerError, "remove failed: %v", err)
		}
		result = append(result, DeleteReplicaSetResult{
			ReplicaSetId: rs.ReplicaSetId,
			Name:         rs.Spec.Name,
			Namespace:    rs.Spec.Namespace,
		})
	}
	if len(result) == 0 {
		return nil, statusError(http.StatusNotFound, "replicaset not found")
	}
	return result, nil
}

func (s *ResourceService) deletePod(rawBytes []byte) ([]DeletePodResult, error) {
	manifest, err := decodeSinglePodManifest(rawBytes)
	if err != nil {
		return nil, err
	}
	list, err := s.psmHandler.GetPodList()
	if err != nil {
		return nil, statusError(http.StatusInternalServerError, "list failed: %v", err)
	}
	var result []DeletePodResult
	for _, p := range list {
		if p.Name != manifest.Name || p.Namespace != manifest.Namespace {
			continue
		}
		if _, err := s.podHandler.Remove(p.PodId); err != nil {
			return nil, statusError(http.StatusInternalServerError, "remove failed: %v", err)
		}
		result = append(result, DeletePodResult{
			PodId:     p.PodId,
			Name:      p.Name,
			Namespace: p.Namespace,
		})
	}
	if len(result) == 0 {
		return nil, statusError(http.StatusNotFound, "pod not found")
	}
	return result, nil
}

func decodeSinglePodManifest(rawBytes []byte) (pod.PodManifest, error) {
	manifests, err := pod.DecodeK8sManifests(rawBytes)
	if err != nil || len(manifests) == 0 {
		return pod.PodManifest{}, statusMessage(http.StatusBadRequest, invalidYAMLErrorMessage(err))
	}
	return manifests[0], nil
}

func (s *ResourceService) removeDeploymentById(deploymentId string) error {
	return s.RemoveDeploymentById(deploymentId)
}

func (s *ResourceService) RemoveDeploymentById(deploymentId string) error {
	deploy, err := s.psmHandler.GetDeployment(deploymentId)
	if err != nil {
		return err
	}
	if err := s.psmHandler.RemoveDeployment(deploymentId); err != nil {
		return err
	}
	if deploy.Spec.ReplicaSetId != "" {
		if err := s.RemoveReplicaSetById(deploy.Spec.ReplicaSetId); err != nil && !strings.Contains(err.Error(), "not found") {
			return err
		}
	}
	inUse, err := s.psmHandler.IsTemplateReferenced(deploy.Spec.TemplateId)
	if err == nil && !inUse {
		_ = s.psmHandler.RemovePodTemplate(deploy.Spec.TemplateId)
	}
	return nil
}

func (s *ResourceService) removeReplicaSetById(replicaSetId string) error {
	return s.RemoveReplicaSetById(replicaSetId)
}

func (s *ResourceService) RemoveReplicaSetById(replicaSetId string) error {
	rs, err := s.psmHandler.GetReplicaSet(replicaSetId)
	if err != nil {
		return err
	}
	if err := s.psmHandler.RemoveReplicaSet(replicaSetId); err != nil {
		return err
	}
	inUse, err := s.psmHandler.IsTemplateReferenced(rs.Spec.TemplateId)
	if err == nil && !inUse {
		_ = s.psmHandler.RemovePodTemplate(rs.Spec.TemplateId)
	}
	if err := s.removePodsByTemplateId(rs.Spec.TemplateId); err != nil {
		return err
	}
	return nil
}

func (s *ResourceService) removePodsByTemplateId(templateId string) error {
	pods, err := s.psmHandler.GetPodList()
	if err != nil {
		return err
	}
	for _, p := range pods {
		if p.TemplateId != templateId {
			continue
		}
		if _, err := s.podHandler.Remove(p.PodId); err != nil {
			return err
		}
	}
	return nil
}
