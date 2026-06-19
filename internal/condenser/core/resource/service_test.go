package resource

import (
	"errors"
	"net/http"
	"testing"

	corenamespace "raind/internal/condenser/core/namespace"
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
kind: ConfigMap
metadata:
  name: app-config
`))

	require.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, ErrorStatus(err, http.StatusOK))
	assert.Contains(t, err.Error(), "unsupported kind: ConfigMap")
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
