package resource

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	coreConfigMap "raind/internal/condenser/core/configmap"
	"raind/internal/condenser/core/container"
	coreIngress "raind/internal/condenser/core/ingress"
	corenamespace "raind/internal/condenser/core/namespace"
	coreNetworkPolicy "raind/internal/condenser/core/networkpolicy"
	"raind/internal/condenser/core/pod"
	coreSecret "raind/internal/condenser/core/secret"
	coreService "raind/internal/condenser/core/service"
	"raind/internal/condenser/store/cfm"
	"raind/internal/condenser/store/ism"
	"raind/internal/condenser/store/psm"
	"raind/internal/condenser/store/sec"
	"raind/internal/condenser/store/ssm"
	"raind/internal/condenser/utils"

	"gopkg.in/yaml.v3"
)

func NewResourceService() *ResourceService {
	return &ResourceService{
		podHandler:       pod.NewPodService(),
		containerHandler: container.NewContaierService(),
		namespaceHandler: corenamespace.NewNamespaceService(),
		psmHandler:       psm.NewPsmManager(psm.NewPsmStore(utils.PsmStorePath)),
		ssmHandler:       ssm.NewSsmManager(ssm.NewSsmStore(utils.SsmStorePath)),
		ismHandler:       ism.NewIsmManager(ism.NewIsmStore(utils.IsmStorePath)),
		cfmHandler:       cfm.NewCfmManager(cfm.NewCfmStore(utils.CfmStorePath)),
		secHandler:       sec.NewSecManager(sec.NewSecStore(utils.SecStorePath)),
	}
}

type ResourceService struct {
	podHandler       pod.PodServiceHandler
	containerHandler container.ContainerServiceHandler
	namespaceHandler corenamespace.NamespaceServiceHandler
	psmHandler       psm.PsmHandler
	ssmHandler       ssm.SsmHandler
	ismHandler       ism.IsmHandler
	cfmHandler       cfm.CfmHandler
	secHandler       sec.SecHandler
}

func (s *ResourceService) Apply(body []byte) (ApplyResult, error) {
	var result ApplyResult
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
			return ApplyResult{}, statusMessage(http.StatusBadRequest, invalidYAMLErrorMessage(err))
		}
		if len(raw) == 0 {
			continue
		}
		rawBytes, err := yaml.Marshal(raw)
		if err != nil {
			rollbackApplied()
			return ApplyResult{}, statusMessage(http.StatusBadRequest, invalidYAMLErrorMessage(err))
		}
		header, err := decodeHeader(rawBytes)
		if err != nil {
			rollbackApplied()
			return ApplyResult{}, statusMessage(http.StatusBadRequest, err.Error())
		}
		result.Warnings = append(result.Warnings, collectHeaderWarnings(header, raw)...)

		switch header.Kind {
		case "Namespace":
			manifest, err := decodeNamespaceManifest(rawBytes)
			if err != nil {
				rollbackApplied()
				return ApplyResult{}, statusMessage(http.StatusBadRequest, invalidYAMLErrorMessage(err))
			}
			info, err := s.namespaceHandler.Create(corenamespace.ServiceCreateModel{
				Name:        manifest.Metadata.Name,
				Labels:      manifest.Metadata.Labels,
				Annotations: manifest.Metadata.Annotations,
			})
			if err != nil {
				rollbackApplied()
				return ApplyResult{}, statusError(http.StatusInternalServerError, "namespace create failed: %v", err)
			}
			rollback = append(rollback, func() {
				_, _ = s.namespaceHandler.Remove(corenamespace.ServiceRemoveModel{Name: info.Name})
			})
			result.Namespaces = append(result.Namespaces, ApplyNamespaceResult{Name: info.Name, Network: info.Network})

		case "Service":
			serviceResult, undo, err := s.applyService(rawBytes)
			if err != nil {
				rollbackApplied()
				return ApplyResult{}, err
			}
			rollback = append(rollback, undo)
			result.Services = append(result.Services, serviceResult)

		case "Ingress":
			ingressResult, undo, err := s.applyIngress(rawBytes)
			if err != nil {
				rollbackApplied()
				return ApplyResult{}, err
			}
			rollback = append(rollback, undo)
			result.Ingresses = append(result.Ingresses, ingressResult)

		case "ConfigMap":
			configMapResult, undo, warnings, err := s.applyConfigMap(rawBytes)
			if err != nil {
				rollbackApplied()
				return ApplyResult{}, err
			}
			rollback = append(rollback, undo)
			result.ConfigMaps = append(result.ConfigMaps, configMapResult)
			result.Warnings = append(result.Warnings, warnings...)

		case "Secret":
			secretResult, undo, err := s.applySecret(rawBytes)
			if err != nil {
				rollbackApplied()
				return ApplyResult{}, err
			}
			rollback = append(rollback, undo)
			result.Secrets = append(result.Secrets, secretResult)

		case "NetworkPolicy":
			networkPolicyResult, undo, err := s.applyNetworkPolicy(rawBytes)
			if err != nil {
				rollbackApplied()
				return ApplyResult{}, err
			}
			rollback = append(rollback, undo)
			result.NetworkPolicies = append(result.NetworkPolicies, networkPolicyResult)

		case "Pod", "ReplicaSet", "Deployment":
			if err := s.applyPodResource(rawBytes, &result, &rollback); err != nil {
				rollbackApplied()
				return ApplyResult{}, err
			}

		default:
			rollbackApplied()
			return ApplyResult{}, statusError(http.StatusBadRequest, "unsupported kind: %s", header.Kind)
		}
	}

	return result, nil
}

func (s *ResourceService) applySecret(rawBytes []byte) (ApplySecretResult, func(), error) {
	manifest, err := coreSecret.DecodeK8sSecretManifest(rawBytes)
	if err != nil {
		return ApplySecretResult{}, nil, statusMessage(http.StatusBadRequest, invalidYAMLErrorMessage(err))
	}
	if s.secHandler.IsNameAlreadyUsed(manifest.Name, manifest.Namespace) {
		return ApplySecretResult{}, nil, statusError(http.StatusBadRequest, "name already used by other secret")
	}
	secretId := utils.NewUlid()
	if err := s.secHandler.StoreSecret(secretId, sec.SecretInfo{
		Name:      manifest.Name,
		Namespace: manifest.Namespace,
		Type:      manifest.Type,
		Data:      manifest.Data,
	}); err != nil {
		return ApplySecretResult{}, nil, statusError(http.StatusInternalServerError, "secret store failed: %v", err)
	}
	return ApplySecretResult{
			SecretId:  secretId,
			Name:      manifest.Name,
			Namespace: manifest.Namespace,
		}, func() {
			_ = s.secHandler.RemoveSecret(secretId)
		}, nil
}

func (s *ResourceService) applyNetworkPolicy(rawBytes []byte) (ApplyNetworkPolicyResult, func(), error) {
	manifest, err := coreNetworkPolicy.DecodeK8sNetworkPolicyManifest(rawBytes)
	if err != nil {
		return ApplyNetworkPolicyResult{}, nil, statusMessage(http.StatusBadRequest, invalidYAMLErrorMessage(err))
	}
	info, err := coreNetworkPolicy.NewService().Apply(manifest)
	if err != nil {
		return ApplyNetworkPolicyResult{}, nil, statusError(http.StatusInternalServerError, "networkpolicy apply failed: %v", err)
	}
	return ApplyNetworkPolicyResult{
			NetworkPolicyId: info.NetworkPolicyId,
			Name:            info.Name,
			Namespace:       info.Namespace,
			GeneratedRules:  len(info.GeneratedRuleIds),
		}, func() {
			_, _ = coreNetworkPolicy.NewService().Remove(info.NetworkPolicyId, "")
		}, nil
}

func (s *ResourceService) applyConfigMap(rawBytes []byte) (ApplyConfigMapResult, func(), []Warning, error) {
	manifest, err := coreConfigMap.DecodeK8sConfigMapManifest(rawBytes)
	if err != nil {
		return ApplyConfigMapResult{}, nil, nil, statusMessage(http.StatusBadRequest, invalidYAMLErrorMessage(err))
	}
	if s.cfmHandler.IsNameAlreadyUsed(manifest.Name, manifest.Namespace) {
		return ApplyConfigMapResult{}, nil, nil, statusError(http.StatusBadRequest, "name already used by other configmap")
	}
	configMapId := utils.NewUlid()
	if err := s.cfmHandler.StoreConfigMap(configMapId, cfm.ConfigMapInfo{
		Name:      manifest.Name,
		Namespace: manifest.Namespace,
		Data:      manifest.Data,
	}); err != nil {
		return ApplyConfigMapResult{}, nil, nil, statusError(http.StatusInternalServerError, "configmap store failed: %v", err)
	}
	warnings := make([]Warning, 0, len(manifest.Warnings))
	for _, warning := range manifest.Warnings {
		warnings = append(warnings, Warning{
			Kind:      "ConfigMap",
			Name:      manifest.Name,
			Namespace: manifest.Namespace,
			Field:     warning.Field,
			Message:   warning.Message,
		})
	}
	return ApplyConfigMapResult{
			ConfigMapId: configMapId,
			Name:        manifest.Name,
			Namespace:   manifest.Namespace,
		}, func() {
			_ = s.cfmHandler.RemoveConfigMap(configMapId)
		}, warnings, nil
}

func (s *ResourceService) applyService(rawBytes []byte) (ApplyServiceResult, func(), error) {
	manifest, err := coreService.DecodeK8sServiceManifest(rawBytes)
	if err != nil {
		return ApplyServiceResult{}, nil, statusMessage(http.StatusBadRequest, invalidYAMLErrorMessage(err))
	}
	if manifest.Name == "" || manifest.Namespace == "" {
		return ApplyServiceResult{}, nil, statusError(http.StatusBadRequest, "name and namespace are required")
	}
	if s.ssmHandler.IsNameAlreadyUsed(manifest.Name, manifest.Namespace) {
		return ApplyServiceResult{}, nil, statusError(http.StatusBadRequest, "name already used by other service")
	}
	serviceId := utils.NewUlid()
	if err := s.ssmHandler.StoreService(serviceId, ssm.ServiceInfo{
		Name:      manifest.Name,
		Namespace: manifest.Namespace,
		Type:      manifest.Type,
		ClusterIP: manifest.ClusterIP,
		Selector:  manifest.Selector,
		Ports:     manifest.Ports,
	}); err != nil {
		return ApplyServiceResult{}, nil, statusError(http.StatusInternalServerError, "service store failed: %v", err)
	}
	return ApplyServiceResult{
			ServiceId: serviceId,
			Name:      manifest.Name,
			Namespace: manifest.Namespace,
		}, func() {
			_ = s.ssmHandler.RemoveService(serviceId)
		}, nil
}

func (s *ResourceService) applyIngress(rawBytes []byte) (ApplyIngressResult, func(), error) {
	manifest, err := coreIngress.DecodeK8sIngressManifest(rawBytes)
	if err != nil {
		return ApplyIngressResult{}, nil, statusMessage(http.StatusBadRequest, invalidYAMLErrorMessage(err))
	}
	if s.ismHandler.IsNameAlreadyUsed(manifest.Name, manifest.Namespace) {
		return ApplyIngressResult{}, nil, statusError(http.StatusBadRequest, "name already used by other ingress")
	}
	if len(manifest.TLSHosts) > 0 {
		if err := coreIngress.NewTLSManager().EnsureHosts(manifest.TLSHosts); err != nil {
			return ApplyIngressResult{}, nil, statusError(http.StatusInternalServerError, "ingress tls certificate create failed: %v", err)
		}
	}
	ingressId := utils.NewUlid()
	if err := s.ismHandler.StoreIngress(ingressId, ism.IngressInfo{
		Name:      manifest.Name,
		Namespace: manifest.Namespace,
		Rules:     manifest.Rules,
		TLSHosts:  manifest.TLSHosts,
	}); err != nil {
		return ApplyIngressResult{}, nil, statusError(http.StatusInternalServerError, "ingress store failed: %v", err)
	}
	return ApplyIngressResult{
			IngressId: ingressId,
			Name:      manifest.Name,
			Namespace: manifest.Namespace,
			TLSHosts:  manifest.TLSHosts,
		}, func() {
			_ = s.ismHandler.RemoveIngress(ingressId)
		}, nil
}

func (s *ResourceService) applyPodResource(rawBytes []byte, result *ApplyResult, rollback *[]func()) error {
	manifests, err := pod.DecodeK8sManifests(rawBytes)
	if err != nil || len(manifests) == 0 {
		return statusMessage(http.StatusBadRequest, invalidYAMLErrorMessage(err))
	}
	m := manifests[0]
	if err := s.resolveConfigMapEnv(&m); err != nil {
		return statusMessage(http.StatusBadRequest, err.Error())
	}
	if err := s.resolveSecretEnv(&m); err != nil {
		return statusMessage(http.StatusBadRequest, err.Error())
	}
	if m.Kind == "Deployment" {
		return s.applyDeploymentManifest(m, result, rollback)
	}
	if m.Kind == "ReplicaSet" {
		return s.applyReplicaSetManifest(m, result, rollback)
	}
	return s.applyPodManifest(m, result, rollback)
}

func (s *ResourceService) resolveSecretEnv(m *pod.PodManifest) error {
	if len(m.SecretEnvFrom) == 0 && len(m.SecretEnvKeys) == 0 {
		return nil
	}
	envByContainer := make([]map[string]string, len(m.Containers))
	for i := range m.Containers {
		envByContainer[i] = envListToMap(m.Containers[i].Env)
	}
	for _, ref := range m.SecretEnvFrom {
		if ref.ContainerIndex < 0 || ref.ContainerIndex >= len(m.Containers) {
			return fmt.Errorf("secret envFrom reference has invalid container index")
		}
		secret, err := s.secHandler.GetSecretByName(ref.Name, m.Namespace)
		if err != nil {
			return fmt.Errorf("secret %s/%s not found: %w", m.Namespace, ref.Name, err)
		}
		for k, v := range secret.Data {
			if _, exists := envByContainer[ref.ContainerIndex][k]; !exists {
				envByContainer[ref.ContainerIndex][k] = v
			}
		}
	}
	for _, ref := range m.SecretEnvKeys {
		if ref.ContainerIndex < 0 || ref.ContainerIndex >= len(m.Containers) {
			return fmt.Errorf("secret key reference has invalid container index")
		}
		if ref.Name == "" || ref.Key == "" {
			return fmt.Errorf("secretKeyRef requires name and key")
		}
		secret, err := s.secHandler.GetSecretByName(ref.Name, m.Namespace)
		if err != nil {
			return fmt.Errorf("secret %s/%s not found: %w", m.Namespace, ref.Name, err)
		}
		value, ok := secret.Data[ref.Key]
		if !ok {
			return fmt.Errorf("secret %s/%s key %q not found", m.Namespace, ref.Name, ref.Key)
		}
		envByContainer[ref.ContainerIndex][ref.EnvName] = value
	}
	for i := range m.Containers {
		m.Containers[i].Env = envMapToList(envByContainer[i])
	}
	return nil
}

func (s *ResourceService) resolveConfigMapEnv(m *pod.PodManifest) error {
	if len(m.ConfigMapEnvFrom) == 0 && len(m.ConfigMapEnvKeys) == 0 {
		return nil
	}
	envByContainer := make([]map[string]string, len(m.Containers))
	for i := range m.Containers {
		envByContainer[i] = envListToMap(m.Containers[i].Env)
	}
	for _, ref := range m.ConfigMapEnvFrom {
		if ref.ContainerIndex < 0 || ref.ContainerIndex >= len(m.Containers) {
			return fmt.Errorf("configmap envFrom reference has invalid container index")
		}
		cm, err := s.cfmHandler.GetConfigMapByName(ref.Name, m.Namespace)
		if err != nil {
			return fmt.Errorf("configmap %s/%s not found: %w", m.Namespace, ref.Name, err)
		}
		for k, v := range cm.Data {
			if _, exists := envByContainer[ref.ContainerIndex][k]; !exists {
				envByContainer[ref.ContainerIndex][k] = v
			}
		}
	}
	for _, ref := range m.ConfigMapEnvKeys {
		if ref.ContainerIndex < 0 || ref.ContainerIndex >= len(m.Containers) {
			return fmt.Errorf("configmap key reference has invalid container index")
		}
		if ref.Name == "" || ref.Key == "" {
			return fmt.Errorf("configmapKeyRef requires name and key")
		}
		cm, err := s.cfmHandler.GetConfigMapByName(ref.Name, m.Namespace)
		if err != nil {
			return fmt.Errorf("configmap %s/%s not found: %w", m.Namespace, ref.Name, err)
		}
		value, ok := cm.Data[ref.Key]
		if !ok {
			return fmt.Errorf("configmap %s/%s key %q not found", m.Namespace, ref.Name, ref.Key)
		}
		envByContainer[ref.ContainerIndex][ref.EnvName] = value
	}
	for i := range m.Containers {
		m.Containers[i].Env = envMapToList(envByContainer[i])
	}
	return nil
}

func envListToMap(env []string) map[string]string {
	out := map[string]string{}
	for _, kv := range env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 || parts[0] == "" {
			continue
		}
		out[parts[0]] = parts[1]
	}
	return out
}

func envMapToList(env map[string]string) []string {
	out := make([]string, 0, len(env))
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := env[k]
		out = append(out, k+"="+v)
	}
	return out
}

func (s *ResourceService) applyDeploymentManifest(m pod.PodManifest, result *ApplyResult, rollback *[]func()) error {
	if err := s.resolveManifestNetworks(&m); err != nil {
		return statusMessage(http.StatusBadRequest, err.Error())
	}
	if err := s.ensureResourceNameAvailable(m.Name, m.Namespace); err != nil {
		return statusMessage(http.StatusBadRequest, err.Error())
	}
	templateId, err := s.storeTemplate(m)
	if err != nil {
		return err
	}
	*rollback = append(*rollback, func() {
		_ = s.psmHandler.RemovePodTemplate(templateId)
	})
	deploymentId := utils.NewUlid()
	if err := s.psmHandler.StoreDeployment(deploymentId, psm.DeploymentSpec{
		Name:       m.Name,
		Namespace:  m.Namespace,
		Replicas:   m.Replicas,
		TemplateId: templateId,
		Selector:   m.Selector,
	}); err != nil {
		return statusError(http.StatusInternalServerError, "deployment store failed: %v", err)
	}
	*rollback = append(*rollback, func() {
		_ = s.removeDeploymentById(deploymentId)
	})
	result.Deployments = append(result.Deployments, ApplyDeploymentResult{
		DeploymentId: deploymentId,
		Namespace:    m.Namespace,
		Name:         m.Name,
		Replicas:     m.Replicas,
	})
	return nil
}

func (s *ResourceService) applyReplicaSetManifest(m pod.PodManifest, result *ApplyResult, rollback *[]func()) error {
	if err := s.resolveManifestNetworks(&m); err != nil {
		return statusMessage(http.StatusBadRequest, err.Error())
	}
	if err := s.ensureResourceNameAvailable(m.Name, m.Namespace); err != nil {
		return statusMessage(http.StatusBadRequest, err.Error())
	}
	templateId, err := s.storeTemplate(m)
	if err != nil {
		return err
	}
	*rollback = append(*rollback, func() {
		_ = s.psmHandler.RemovePodTemplate(templateId)
	})
	replicaSetId := utils.NewUlid()
	if err := s.psmHandler.StoreReplicaSet(replicaSetId, psm.ReplicaSetSpec{
		Name:       m.Name,
		Namespace:  m.Namespace,
		Replicas:   m.Replicas,
		TemplateId: templateId,
		Selector:   m.Selector,
	}); err != nil {
		return statusError(http.StatusInternalServerError, "replicaset store failed: %v", err)
	}
	*rollback = append(*rollback, func() {
		_ = s.removeReplicaSetById(replicaSetId)
	})
	result.ReplicaSets = append(result.ReplicaSets, ApplyReplicaSetResult{
		ReplicaSetId: replicaSetId,
		Namespace:    m.Namespace,
		Name:         m.Name,
	})
	return nil
}

func (s *ResourceService) applyPodManifest(m pod.PodManifest, result *ApplyResult, rollback *[]func()) error {
	if err := s.resolveManifestNetworks(&m); err != nil {
		return statusMessage(http.StatusBadRequest, err.Error())
	}
	podId, err := s.podHandler.Create(pod.ServiceCreateModel{
		Name:        m.Name,
		Namespace:   m.Namespace,
		Labels:      m.Labels,
		Annotations: m.Annotations,
		Rootless:    m.Rootless,
		Containers:  m.Containers,
	})
	if err != nil {
		return statusError(http.StatusInternalServerError, "pod create failed: %v", err)
	}
	*rollback = append(*rollback, func() {
		_, _ = s.podHandler.Remove(podId)
	})

	if _, err := s.podHandler.Start(podId); err != nil {
		_, _ = s.podHandler.Remove(podId)
		return statusError(http.StatusInternalServerError, "pod start failed: %v", err)
	}

	containers, err := s.containerHandler.GetContainersByPodId(podId)
	if err != nil {
		_, _ = s.podHandler.Remove(podId)
		return statusError(http.StatusInternalServerError, "container list failed: %v", err)
	}
	containerIds := make([]string, 0, len(containers))
	for _, c := range containers {
		if strings.HasPrefix(c.Name, utils.PodInfraContainerNamePrefix) {
			continue
		}
		containerIds = append(containerIds, c.ContainerId)
	}

	result.Pods = append(result.Pods, ApplyPodResult{
		PodId:        podId,
		Namespace:    m.Namespace,
		Name:         m.Name,
		ContainerIds: containerIds,
	})
	return nil
}

func (s *ResourceService) storeTemplate(m pod.PodManifest) (string, error) {
	templateId := utils.NewUlid()
	if err := s.psmHandler.StorePodTemplate(templateId, psm.PodTemplateSpec{
		Name:        m.Name,
		Namespace:   m.Namespace,
		Labels:      m.Labels,
		Annotations: m.Annotations,
		Rootless:    m.Rootless,
		Containers:  m.Containers,
	}); err != nil {
		return "", statusError(http.StatusInternalServerError, "template store failed: %v", err)
	}
	return templateId, nil
}

func (s *ResourceService) ensureResourceNameAvailable(name, namespace string) error {
	if name == "" || namespace == "" {
		return nil
	}
	if s.psmHandler.IsNameAlreadyUsed(name, namespace) {
		return errNameAlreadyUsed(name, namespace)
	}
	replicaSets, err := s.psmHandler.GetReplicaSetList()
	if err != nil {
		return err
	}
	for _, rs := range replicaSets {
		if rs.Spec.Name == name && rs.Spec.Namespace == namespace {
			return errNameAlreadyUsed(name, namespace)
		}
	}
	deployments, err := s.psmHandler.GetDeploymentList()
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

func (s *ResourceService) resolveManifestNetworks(m *pod.PodManifest) error {
	networkName, err := s.namespaceHandler.ResolveNetwork(m.Namespace)
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
