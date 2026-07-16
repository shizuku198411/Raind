package resource

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"

	corenamespace "raind/internal/condenser/core/namespace"
	"raind/internal/condenser/core/pod"
	"raind/internal/condenser/store/cfm"
	"raind/internal/condenser/store/psm"
	"raind/internal/condenser/store/sec"
	"raind/internal/condenser/store/ssm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyProcessesMultiDocumentManifestsInOrder(t *testing.T) {
	var events []string
	namespaceHandler := &fakeNamespaceHandler{events: &events}
	ssmHandler := &fakeSsmHandler{events: &events}
	service := &ResourceService{
		namespaceHandler: namespaceHandler,
		ssmHandler:       ssmHandler,
	}

	result, err := service.Apply([]byte(`
apiVersion: v1
kind: Namespace
metadata:
  name: demo
---
apiVersion: v1
kind: Service
metadata:
  name: web
  namespace: demo
spec:
  selector:
    app: web
  ports:
    - port: 80
      targetPort: 8080
`))

	require.NoError(t, err)
	require.Len(t, result.Namespaces, 1)
	require.Len(t, result.Services, 1)
	assert.Equal(t, []string{"namespace:create:demo", "service:store:web"}, events)
	assert.Equal(t, "demo", result.Namespaces[0].Name)
	assert.Equal(t, "web", result.Services[0].Name)
	assert.Empty(t, result.Warnings)
}

func TestPodTemplateSpecEqualTreatsRuntimeDefaultSecurityProfileAsEmpty(t *testing.T) {
	stored := psm.PodTemplateSpec{
		Name:      "web",
		Namespace: "demo",
		Containers: []psm.ContainerTemplateSpec{{
			Name:            "nginx",
			Image:           "nginx:alpine",
			Network:         "rns123",
			SecurityProfile: "default",
		}},
	}
	desired := psm.PodTemplateSpec{
		Name:      "web",
		Namespace: "demo",
		Containers: []psm.ContainerTemplateSpec{{
			Name:    "nginx",
			Image:   "nginx:alpine",
			Network: "rns123",
		}},
	}

	assert.True(t, podTemplateSpecEqual(stored, desired))
}

func TestPodTemplateSpecEqualDetectsExplicitSecurityProfileChange(t *testing.T) {
	stored := psm.PodTemplateSpec{
		Name:      "web",
		Namespace: "demo",
		Containers: []psm.ContainerTemplateSpec{{
			Name:            "nginx",
			Image:           "nginx:alpine",
			Network:         "rns123",
			SecurityProfile: "restricted",
		}},
	}
	desired := psm.PodTemplateSpec{
		Name:      "web",
		Namespace: "demo",
		Containers: []psm.ContainerTemplateSpec{{
			Name:    "nginx",
			Image:   "nginx:alpine",
			Network: "rns123",
		}},
	}

	assert.False(t, podTemplateSpecEqual(stored, desired))
}

func TestApplyRollsBackAlreadyAppliedDocumentsOnFailure(t *testing.T) {
	var events []string
	namespaceHandler := &fakeNamespaceHandler{events: &events}
	ssmHandler := &fakeSsmHandler{
		events:   &events,
		storeErr: errors.New("store unavailable"),
	}
	service := &ResourceService{
		namespaceHandler: namespaceHandler,
		ssmHandler:       ssmHandler,
	}

	result, err := service.Apply([]byte(`
apiVersion: v1
kind: Namespace
metadata:
  name: demo
---
apiVersion: v1
kind: Service
metadata:
  name: web
  namespace: demo
spec:
  ports:
    - port: 80
`))

	require.Error(t, err)
	assert.Equal(t, http.StatusInternalServerError, ErrorStatus(err, http.StatusOK))
	assert.Empty(t, result.Namespaces)
	assert.Equal(t, []string{"namespace:create:demo", "service:store:web", "namespace:remove:demo"}, events)
}

func TestApplyRejectsUnsupportedKind(t *testing.T) {
	service := &ResourceService{}

	_, err := service.Apply([]byte(`
apiVersion: v1
kind: Widget
metadata:
  name: app-widget
`))

	require.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, ErrorStatus(err, http.StatusOK))
	assert.Contains(t, err.Error(), "unsupported kind: Widget")
}

func TestApplyReturnsManifestWarnings(t *testing.T) {
	service := &ResourceService{
		namespaceHandler: &fakeNamespaceHandler{},
	}

	result, err := service.Apply([]byte(`
apiVersion: v1
kind: Namespace
metadata:
  name: demo
  generateName: ignored-
`))

	require.NoError(t, err)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, Warning{
		Kind:    "Namespace",
		Name:    "demo",
		Field:   "metadata.generateName",
		Message: "metadata.generateName is ignored; set metadata.name explicitly",
	}, result.Warnings[0])
}

func TestApplyStoresConfigMap(t *testing.T) {
	cfmHandler := newFakeCfmHandler()
	service := &ResourceService{cfmHandler: cfmHandler}

	result, err := service.Apply([]byte(`
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
  namespace: demo
data:
  APP_ENV: test
`))

	require.NoError(t, err)
	require.Len(t, result.ConfigMaps, 1)
	assert.Equal(t, "app-config", result.ConfigMaps[0].Name)
	stored, err := cfmHandler.GetConfigMapByName("app-config", "demo")
	require.NoError(t, err)
	assert.Equal(t, "test", stored.Data["APP_ENV"])
}

func TestResolveConfigMapEnv(t *testing.T) {
	cfmHandler := newFakeCfmHandler()
	require.NoError(t, cfmHandler.StoreConfigMap("cm-1", cfm.ConfigMapInfo{
		Name:      "app-config",
		Namespace: "demo",
		Data: map[string]string{
			"APP_ENV":     "test",
			"LOG_LEVEL":   "info",
			"OVERRIDE_ME": "from-envfrom",
		},
	}))
	service := &ResourceService{cfmHandler: cfmHandler}
	manifest := podManifestForConfigMapEnv()

	require.NoError(t, service.resolveConfigMapEnv(&manifest))
	require.Len(t, manifest.Containers, 1)
	assert.ElementsMatch(t, []string{
		"APP_ENV=test",
		"LOG_LEVEL=info",
		"OVERRIDE_ME=explicit",
		"SINGLE_KEY=test",
	}, manifest.Containers[0].Env)
}

func TestApplyStoresSecretWithoutReturningValues(t *testing.T) {
	secHandler := newFakeSecHandler()
	service := &ResourceService{secHandler: secHandler}

	result, err := service.Apply([]byte(`
apiVersion: v1
kind: Secret
metadata:
  name: db-secret
  namespace: demo
type: Opaque
stringData:
  DB_PASSWORD: super-secret
`))

	require.NoError(t, err)
	require.Len(t, result.Secrets, 1)
	assert.Equal(t, "db-secret", result.Secrets[0].Name)
	assert.NotContains(t, fmt.Sprintf("%+v", result), "super-secret")
	stored, err := secHandler.GetSecretByName("db-secret", "demo")
	require.NoError(t, err)
	assert.Equal(t, "super-secret", stored.Data["DB_PASSWORD"])
}

func TestApplyRollsOutExistingDeploymentWhenTemplateChanges(t *testing.T) {
	manager := psm.NewPsmManager(psm.NewPsmStore(filepath.Join(t.TempDir(), "psm.json")))
	require.NoError(t, manager.StorePodTemplate("tpl-old", psm.PodTemplateSpec{
		Name:      "web",
		Namespace: "demo",
		Containers: []psm.ContainerTemplateSpec{{
			Name:    "app",
			Image:   "nginx:1.25",
			Network: "rns-demo",
		}},
	}))
	require.NoError(t, manager.StoreDeployment("deploy-1", psm.DeploymentSpec{
		Name:         "web",
		Namespace:    "demo",
		Replicas:     2,
		TemplateId:   "tpl-old",
		ReplicaSetId: "rs-1",
		Selector:     map[string]string{"app": "web"},
	}))
	require.NoError(t, manager.StoreReplicaSet("rs-1", psm.ReplicaSetSpec{
		Name:       "web",
		Namespace:  "demo",
		Replicas:   2,
		TemplateId: "tpl-old",
		Selector:   map[string]string{"app": "web"},
	}))
	require.NoError(t, manager.StorePod(psm.StorePodRequest{
		PodId:      "pod-old",
		TemplateId: "tpl-old",
		OwnerKind:  psm.OwnerKindReplicaSet,
		OwnerId:    "rs-1",
		Name:       "web-old",
		Namespace:  "demo",
		State:      psm.PodStateRunning,
	}))
	podHandler := &fakeRolloutPodHandler{}
	service := &ResourceService{
		namespaceHandler: &fakeNamespaceHandler{},
		psmHandler:       manager,
		podHandler:       podHandler,
	}

	result, err := service.Apply([]byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: demo
spec:
  replicas: 3
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
        - name: app
          image: nginx:1.26
`))

	require.NoError(t, err)
	require.Len(t, result.Deployments, 1)
	assert.Equal(t, "deploy-1", result.Deployments[0].DeploymentId)
	assert.Equal(t, 3, result.Deployments[0].Replicas)
	assert.Equal(t, []string{"pod-old"}, podHandler.removed)

	deploy, err := manager.GetDeployment("deploy-1")
	require.NoError(t, err)
	assert.NotEqual(t, "tpl-old", deploy.Spec.TemplateId)
	assert.Equal(t, 3, deploy.Spec.Replicas)
	rs, err := manager.GetReplicaSet("rs-1")
	require.NoError(t, err)
	assert.Equal(t, deploy.Spec.TemplateId, rs.Spec.TemplateId)
	assert.Equal(t, 3, rs.Spec.Replicas)
	tpl, err := manager.GetPodTemplate(deploy.Spec.TemplateId)
	require.NoError(t, err)
	require.Len(t, tpl.Spec.Containers, 1)
	assert.Equal(t, "nginx:1.26", tpl.Spec.Containers[0].Image)
	_, err = manager.GetPodTemplate("tpl-old")
	assert.Error(t, err)
}

func TestApplyExistingDeploymentNoopsWhenManifestUnchanged(t *testing.T) {
	manager := psm.NewPsmManager(psm.NewPsmStore(filepath.Join(t.TempDir(), "psm.json")))
	require.NoError(t, manager.StorePodTemplate("tpl-existing", psm.PodTemplateSpec{
		Name:      "web",
		Namespace: "demo",
		Labels:    map[string]string{"app": "web"},
		Containers: []psm.ContainerTemplateSpec{{
			Name:    "app",
			Image:   "nginx:1.25",
			Network: "rns-demo",
		}},
	}))
	require.NoError(t, manager.StoreDeployment("deploy-1", psm.DeploymentSpec{
		Name:         "web",
		Namespace:    "demo",
		Replicas:     2,
		TemplateId:   "tpl-existing",
		ReplicaSetId: "rs-1",
		Selector:     map[string]string{"app": "web"},
	}))
	require.NoError(t, manager.StoreReplicaSet("rs-1", psm.ReplicaSetSpec{
		Name:       "web",
		Namespace:  "demo",
		Replicas:   2,
		TemplateId: "tpl-existing",
		Selector:   map[string]string{"app": "web"},
	}))
	podHandler := &fakeRolloutPodHandler{}
	service := &ResourceService{
		namespaceHandler: &fakeNamespaceHandler{},
		psmHandler:       manager,
		podHandler:       podHandler,
	}

	result, err := service.Apply([]byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: demo
spec:
  replicas: 2
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
        - name: app
          image: nginx:1.25
`))

	require.NoError(t, err)
	require.Len(t, result.Deployments, 1)
	assert.Equal(t, "deploy-1", result.Deployments[0].DeploymentId)
	assert.Empty(t, podHandler.removed)
	deploy, err := manager.GetDeployment("deploy-1")
	require.NoError(t, err)
	assert.Equal(t, "tpl-existing", deploy.Spec.TemplateId)
}

func TestResolveSecretEnv(t *testing.T) {
	secHandler := newFakeSecHandler()
	require.NoError(t, secHandler.StoreSecret("secret-1", sec.SecretInfo{
		Name:      "db-secret",
		Namespace: "demo",
		Type:      sec.SecretTypeOpaque,
		Data: map[string]string{
			"DB_PASSWORD": "super-secret",
			"API_TOKEN":   "token-value",
			"OVERRIDE_ME": "from-secret",
		},
	}))
	service := &ResourceService{secHandler: secHandler}
	manifest := podManifestForSecretEnv()

	require.NoError(t, service.resolveSecretEnv(&manifest))
	require.Len(t, manifest.Containers, 1)
	assert.ElementsMatch(t, []string{
		"API_TOKEN=token-value",
		"DB_PASSWORD=super-secret",
		"OVERRIDE_ME=explicit",
		"SINGLE_SECRET=super-secret",
	}, manifest.Containers[0].Env)
}

func TestDeleteReturnsManifestWarnings(t *testing.T) {
	service := &ResourceService{
		namespaceHandler: &fakeNamespaceHandler{},
	}

	result, err := service.Delete([]byte(`
apiVersion: v1
kind: Namespace
metadata:
  name: demo
  generateName: ignored-
`))

	require.NoError(t, err)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "metadata.generateName", result.Warnings[0].Field)
}

func TestDeleteSkipsMissingChildResourceAndStillRemovesNamespace(t *testing.T) {
	var events []string
	service := &ResourceService{
		namespaceHandler: &fakeNamespaceHandler{events: &events},
		psmHandler:       psm.NewPsmManager(psm.NewPsmStore(filepath.Join(t.TempDir(), "psm.json"))),
	}

	result, err := service.Delete([]byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: missing-web
  namespace: demo
---
apiVersion: v1
kind: Namespace
metadata:
  name: demo
`))

	require.NoError(t, err)
	assert.Equal(t, []string{"namespace:remove:demo"}, events)
	require.Len(t, result.Warnings, 1)
	assert.Equal(t, "Deployment", result.Warnings[0].Kind)
	assert.Equal(t, "missing-web", result.Warnings[0].Name)
	assert.Equal(t, "demo", result.Warnings[0].Namespace)
	assert.Contains(t, result.Warnings[0].Message, "skipped")
	require.Len(t, result.Namespaces, 1)
	assert.Equal(t, "demo", result.Namespaces[0].Name)
}

type fakeNamespaceHandler struct {
	events     *[]string
	namespaces map[string]corenamespace.NamespaceInfo
}

func (h *fakeNamespaceHandler) Create(model corenamespace.ServiceCreateModel) (corenamespace.NamespaceInfo, error) {
	h.record("namespace:create:" + model.Name)
	return corenamespace.NamespaceInfo{
		Name:    model.Name,
		Network: "rns-" + model.Name,
	}, nil
}

func (h *fakeNamespaceHandler) Remove(model corenamespace.ServiceRemoveModel) (string, error) {
	h.record("namespace:remove:" + model.Name)
	return model.Name, nil
}

func (h *fakeNamespaceHandler) Get(name string) (corenamespace.NamespaceInfo, error) {
	if h.namespaces != nil {
		if ns, ok := h.namespaces[name]; ok {
			return ns, nil
		}
	}
	return corenamespace.NamespaceInfo{}, errors.New("namespace not found")
}

func (h *fakeNamespaceHandler) List() ([]corenamespace.NamespaceInfo, error) {
	return nil, nil
}

func (h *fakeNamespaceHandler) ResolveNetwork(name string) (string, error) {
	return "rns-" + name, nil
}

func (h *fakeNamespaceHandler) record(event string) {
	if h.events != nil {
		*h.events = append(*h.events, event)
	}
}

type fakeSsmHandler struct {
	events   *[]string
	storeErr error
	services []ssm.ServiceInfo
}

type fakeCfmHandler struct {
	items map[string]cfm.ConfigMapInfo
}

type fakeSecHandler struct {
	items map[string]sec.SecretInfo
}

type fakeRolloutPodHandler struct {
	removed []string
}

func (h *fakeRolloutPodHandler) Create(pod.ServiceCreateModel) (string, error) {
	return "pod-new", nil
}

func (h *fakeRolloutPodHandler) RecreateFromTemplate(string) (string, error) {
	return "pod-recreated", nil
}

func (h *fakeRolloutPodHandler) CreateFromTemplate(string, string) (string, error) {
	return "pod-recreated", nil
}

func (h *fakeRolloutPodHandler) Start(podId string) (string, error) {
	return podId, nil
}

func (h *fakeRolloutPodHandler) Stop(podId string) (string, error) {
	return podId, nil
}

func (h *fakeRolloutPodHandler) Remove(podId string) (string, error) {
	h.removed = append(h.removed, podId)
	return podId, nil
}

func (h *fakeRolloutPodHandler) GetPodList() ([]pod.PodState, error) {
	return nil, nil
}

func (h *fakeRolloutPodHandler) GetPodById(string) (pod.PodState, error) {
	return pod.PodState{}, nil
}

func newFakeSecHandler() *fakeSecHandler {
	return &fakeSecHandler{items: map[string]sec.SecretInfo{}}
}

func (h *fakeSecHandler) StoreSecret(secretId string, spec sec.SecretInfo) error {
	spec.SecretId = secretId
	if spec.Namespace == "" {
		spec.Namespace = "default"
	}
	if spec.Type == "" {
		spec.Type = sec.SecretTypeOpaque
	}
	if spec.Data == nil {
		spec.Data = map[string]string{}
	}
	h.items[secretId] = spec
	return nil
}

func (h *fakeSecHandler) GetSecretList() ([]sec.SecretInfo, error) {
	out := make([]sec.SecretInfo, 0, len(h.items))
	for _, item := range h.items {
		out = append(out, item)
	}
	return out, nil
}

func (h *fakeSecHandler) GetSecretById(secretId string) (sec.SecretInfo, error) {
	item, ok := h.items[secretId]
	if !ok {
		return sec.SecretInfo{}, errors.New("secret not found")
	}
	return item, nil
}

func (h *fakeSecHandler) GetSecretByName(name, namespace string) (sec.SecretInfo, error) {
	if namespace == "" {
		namespace = "default"
	}
	for _, item := range h.items {
		if item.Name == name && item.Namespace == namespace {
			return item, nil
		}
	}
	return sec.SecretInfo{}, errors.New("secret not found")
}

func (h *fakeSecHandler) RemoveSecret(secretId string) error {
	if _, ok := h.items[secretId]; !ok {
		return errors.New("secret not found")
	}
	delete(h.items, secretId)
	return nil
}

func (h *fakeSecHandler) IsNameAlreadyUsed(name, namespace string) bool {
	if namespace == "" {
		namespace = "default"
	}
	for _, item := range h.items {
		if item.Name == name && item.Namespace == namespace {
			return true
		}
	}
	return false
}

func newFakeCfmHandler() *fakeCfmHandler {
	return &fakeCfmHandler{items: map[string]cfm.ConfigMapInfo{}}
}

func (h *fakeCfmHandler) StoreConfigMap(configMapId string, spec cfm.ConfigMapInfo) error {
	spec.ConfigMapId = configMapId
	if spec.Namespace == "" {
		spec.Namespace = "default"
	}
	if spec.Data == nil {
		spec.Data = map[string]string{}
	}
	h.items[configMapId] = spec
	return nil
}

func (h *fakeCfmHandler) GetConfigMapList() ([]cfm.ConfigMapInfo, error) {
	out := make([]cfm.ConfigMapInfo, 0, len(h.items))
	for _, item := range h.items {
		out = append(out, item)
	}
	return out, nil
}

func (h *fakeCfmHandler) GetConfigMapById(configMapId string) (cfm.ConfigMapInfo, error) {
	item, ok := h.items[configMapId]
	if !ok {
		return cfm.ConfigMapInfo{}, errors.New("configmap not found")
	}
	return item, nil
}

func (h *fakeCfmHandler) GetConfigMapByName(name, namespace string) (cfm.ConfigMapInfo, error) {
	if namespace == "" {
		namespace = "default"
	}
	for _, item := range h.items {
		if item.Name == name && item.Namespace == namespace {
			return item, nil
		}
	}
	return cfm.ConfigMapInfo{}, errors.New("configmap not found")
}

func (h *fakeCfmHandler) RemoveConfigMap(configMapId string) error {
	if _, ok := h.items[configMapId]; !ok {
		return errors.New("configmap not found")
	}
	delete(h.items, configMapId)
	return nil
}

func (h *fakeCfmHandler) IsNameAlreadyUsed(name, namespace string) bool {
	if namespace == "" {
		namespace = "default"
	}
	for _, item := range h.items {
		if item.Name == name && item.Namespace == namespace {
			return true
		}
	}
	return false
}

func podManifestForConfigMapEnv() pod.PodManifest {
	return pod.PodManifest{
		Name:      "app",
		Namespace: "demo",
		Containers: []psm.ContainerTemplateSpec{
			{
				Name: "app",
				Env:  []string{"OVERRIDE_ME=explicit"},
			},
		},
		ConfigMapEnvFrom: []pod.ContainerConfigMapRef{
			{ContainerIndex: 0, Name: "app-config"},
		},
		ConfigMapEnvKeys: []pod.ContainerConfigMapKeyRef{
			{ContainerIndex: 0, EnvName: "SINGLE_KEY", Name: "app-config", Key: "APP_ENV"},
		},
	}
}

func podManifestForSecretEnv() pod.PodManifest {
	return pod.PodManifest{
		Name:      "app",
		Namespace: "demo",
		Containers: []psm.ContainerTemplateSpec{
			{
				Name: "app",
				Env:  []string{"OVERRIDE_ME=explicit"},
			},
		},
		SecretEnvFrom: []pod.ContainerSecretRef{
			{ContainerIndex: 0, Name: "db-secret"},
		},
		SecretEnvKeys: []pod.ContainerSecretKeyRef{
			{ContainerIndex: 0, EnvName: "SINGLE_SECRET", Name: "db-secret", Key: "DB_PASSWORD"},
		},
	}
}

func (h *fakeSsmHandler) StoreService(serviceId string, spec ssm.ServiceInfo) error {
	h.record("service:store:" + spec.Name)
	if h.storeErr != nil {
		return h.storeErr
	}
	spec.ServiceId = serviceId
	h.services = append(h.services, spec)
	return nil
}

func (h *fakeSsmHandler) GetServiceList() ([]ssm.ServiceInfo, error) {
	return h.services, nil
}

func (h *fakeSsmHandler) GetServiceById(serviceId string) (ssm.ServiceInfo, error) {
	for _, service := range h.services {
		if service.ServiceId == serviceId {
			return service, nil
		}
	}
	return ssm.ServiceInfo{}, errors.New("service not found")
}

func (h *fakeSsmHandler) RemoveService(serviceId string) error {
	h.record("service:remove:" + serviceId)
	return nil
}

func (h *fakeSsmHandler) IsNameAlreadyUsed(name, namespace string) bool {
	for _, service := range h.services {
		if service.Name == name && service.Namespace == namespace {
			return true
		}
	}
	return false
}

func (h *fakeSsmHandler) record(event string) {
	if h.events != nil {
		*h.events = append(*h.events, event)
	}
}

func TestSortDeleteManifestDocumentsUsesDependencyAwareOrder(t *testing.T) {
	docs := []deleteManifestDocument{
		{Header: Header{Kind: "Namespace"}},
		{Header: Header{Kind: "PersistentVolumeClaim"}},
		{Header: Header{Kind: "Secret"}},
		{Header: Header{Kind: "ConfigMap"}},
		{Header: Header{Kind: "Service"}},
		{Header: Header{Kind: "NetworkPolicy"}},
		{Header: Header{Kind: "Ingress"}},
		{Header: Header{Kind: "Pod"}},
		{Header: Header{Kind: "ReplicaSet"}},
		{Header: Header{Kind: "Deployment"}},
	}

	ordered := sortDeleteManifestDocuments(docs)

	var kinds []string
	for _, doc := range ordered {
		kinds = append(kinds, doc.Header.Kind)
	}
	assert.Equal(t, []string{
		"Deployment",
		"ReplicaSet",
		"Pod",
		"Ingress",
		"NetworkPolicy",
		"Service",
		"ConfigMap",
		"Secret",
		"PersistentVolumeClaim",
		"Namespace",
	}, kinds)
}

func TestSortDeleteManifestDocumentsKeepsSameKindOrderStable(t *testing.T) {
	docs := []deleteManifestDocument{
		{Header: Header{Kind: "Service", Metadata: struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		}{Name: "web"}}},
		{Header: Header{Kind: "Service", Metadata: struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		}{Name: "db"}}},
	}

	ordered := sortDeleteManifestDocuments(docs)

	require.Len(t, ordered, 2)
	assert.Equal(t, "web", ordered[0].Header.Metadata.Name)
	assert.Equal(t, "db", ordered[1].Header.Metadata.Name)
}
