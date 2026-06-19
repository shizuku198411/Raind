package resource

import (
	"errors"
	"net/http"
	"testing"

	corenamespace "raind/internal/condenser/core/namespace"
	"raind/internal/condenser/core/pod"
	"raind/internal/condenser/store/cfm"
	"raind/internal/condenser/store/psm"
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
kind: Secret
metadata:
  name: app-secret
`))

	require.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, ErrorStatus(err, http.StatusOK))
	assert.Contains(t, err.Error(), "unsupported kind: Secret")
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

type fakeNamespaceHandler struct {
	events *[]string
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
	return corenamespace.NamespaceInfo{Name: name, Network: "rns-" + name}, nil
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
