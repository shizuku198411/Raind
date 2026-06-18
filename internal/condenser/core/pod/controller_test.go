package pod

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"raind/internal/condenser/core/container"
	"raind/internal/condenser/store/psm"
	"raind/internal/condenser/utils"
)

func TestPodControllerRecreatesReplicaSetPodWhenInfraIsStopped(t *testing.T) {
	psmHandler := &fakeControllerPsm{
		replicaSets: []psm.ReplicaSetInfo{{
			ReplicaSetId: "rs-1",
			Spec: psm.ReplicaSetSpec{
				Name:       "demo-web",
				Namespace:  "demo",
				Replicas:   1,
				TemplateId: "tpl-1",
			},
		}},
		templates: []psm.PodTemplateInfo{{TemplateId: "tpl-1"}},
		pods: map[string]psm.PodInfo{
			"pod-1": {
				PodId:      "pod-1",
				TemplateId: "tpl-1",
				Name:       "demo-web-old",
				Namespace:  "demo",
				State:      "running",
			},
		},
	}
	podHandler := &fakeControllerPodService{recreatePodId: "pod-2"}
	containerHandler := &fakeControllerContainerService{
		containersByPod: map[string][]container.ContainerState{
			"pod-1": {
				{ContainerId: "infra-1", Name: utils.PodInfraContainerNamePrefix + "pod-1", PodId: "pod-1", State: "stopped"},
				{ContainerId: "member-1", Name: "web", PodId: "pod-1", State: "running"},
			},
		},
	}
	controller := &PodController{
		psmHandler:       psmHandler,
		podHandler:       podHandler,
		containerHandler: containerHandler,
	}

	if err := controller.reconcileOnce(); err != nil {
		t.Fatalf("reconcileOnce returned error: %v", err)
	}

	if !psmHandler.removedPods["pod-1"] {
		t.Fatalf("expected stale pod to be removed")
	}
	if podHandler.recreatedTemplateId != "tpl-1" {
		t.Fatalf("expected pod to be recreated from template tpl-1, got %q", podHandler.recreatedTemplateId)
	}
	if podHandler.startedPodId != "pod-2" {
		t.Fatalf("expected recreated pod to be started, got %q", podHandler.startedPodId)
	}
	if !containerHandler.stopped["member-1"] || !containerHandler.deleted["member-1"] || !containerHandler.deleted["infra-1"] {
		t.Fatalf("expected old infra and member containers to be cleaned up")
	}
}

func TestPodControllerReplacesUserStoppedReplicaSetPod(t *testing.T) {
	psmHandler := &fakeControllerPsm{
		replicaSets: []psm.ReplicaSetInfo{{
			ReplicaSetId: "rs-1",
			Spec: psm.ReplicaSetSpec{
				Name:       "demo-web",
				Namespace:  "demo",
				Replicas:   2,
				TemplateId: "tpl-1",
			},
		}},
		templates: []psm.PodTemplateInfo{{TemplateId: "tpl-1"}},
		pods: map[string]psm.PodInfo{
			"pod-running": {
				PodId:      "pod-running",
				TemplateId: "tpl-1",
				Name:       "demo-web-running",
				Namespace:  "demo",
				State:      "running",
			},
			"pod-stopped": {
				PodId:         "pod-stopped",
				TemplateId:    "tpl-1",
				Name:          "demo-web-stopped",
				Namespace:     "demo",
				State:         "stopped",
				StoppedByUser: true,
			},
		},
	}
	podHandler := &fakeControllerPodService{createPodId: "pod-replacement"}
	containerHandler := &fakeControllerContainerService{
		containersByPod: map[string][]container.ContainerState{
			"pod-running": {
				{ContainerId: "infra-running", Name: utils.PodInfraContainerNamePrefix + "pod-running", PodId: "pod-running", State: "running"},
				{ContainerId: "member-running", Name: "web", PodId: "pod-running", State: "running"},
			},
			"pod-stopped": {
				{ContainerId: "infra-stopped", Name: utils.PodInfraContainerNamePrefix + "pod-stopped", PodId: "pod-stopped", State: "running"},
				{ContainerId: "member-stopped", Name: "web", PodId: "pod-stopped", State: "stopped"},
			},
		},
	}
	controller := &PodController{
		psmHandler:       psmHandler,
		podHandler:       podHandler,
		containerHandler: containerHandler,
	}

	if err := controller.reconcileOnce(); err != nil {
		t.Fatalf("reconcileOnce returned error: %v", err)
	}

	if !psmHandler.removedPods["pod-stopped"] {
		t.Fatalf("expected user-stopped managed pod to be removed")
	}
	if !containerHandler.stopped["infra-stopped"] || !containerHandler.deleted["infra-stopped"] || !containerHandler.deleted["member-stopped"] {
		t.Fatalf("expected stopped managed pod containers to be cleaned up")
	}
	if podHandler.createdTemplateId != "tpl-1" {
		t.Fatalf("expected replacement pod to be created from template tpl-1, got %q", podHandler.createdTemplateId)
	}
	if podHandler.startedPodId != "pod-replacement" {
		t.Fatalf("expected replacement pod to be started, got %q", podHandler.startedPodId)
	}
}

func TestFilterReplicaSetPodsUsesExplicitOwner(t *testing.T) {
	rs := psm.ReplicaSetInfo{
		ReplicaSetId: "rs-1",
		Spec:         psm.ReplicaSetSpec{TemplateId: "tpl-1"},
	}
	pods := []psm.PodInfo{
		{PodId: "owned", TemplateId: "tpl-1", OwnerKind: psm.OwnerKindReplicaSet, OwnerId: "rs-1"},
		{PodId: "other-owner", TemplateId: "tpl-1", OwnerKind: psm.OwnerKindReplicaSet, OwnerId: "rs-2"},
		{PodId: "unowned-ambiguous", TemplateId: "tpl-1"},
	}

	got := filterReplicaSetPods(rs, pods, map[string]int{"tpl-1": 2})

	if len(got) != 1 || got[0].PodId != "owned" {
		t.Fatalf("expected only explicitly owned pod, got %#v", got)
	}
}

func TestPodControllerTreatsMultipleRunningInfraAsUnhealthy(t *testing.T) {
	controller := &PodController{
		containerHandler: &fakeControllerContainerService{
			containersByPod: map[string][]container.ContainerState{
				"pod-1": {
					{ContainerId: "infra-1", Name: utils.PodInfraContainerNamePrefix + "pod-1", PodId: "pod-1", State: psm.ContainerStateRunning},
					{ContainerId: "infra-2", Name: utils.PodInfraContainerNamePrefix + "pod-1-copy", PodId: "pod-1", State: psm.ContainerStateRunning},
					{ContainerId: "member-1", Name: "web", PodId: "pod-1", State: psm.ContainerStateRunning},
				},
			},
		},
	}

	state, err := controller.getPodInfraState("pod-1")

	if err != nil {
		t.Fatalf("getPodInfraState returned error: %v", err)
	}
	if state != "duplicate" {
		t.Fatalf("expected duplicate infra state, got %q", state)
	}
}

func TestPodControllerRecordsBackoffWhenReplicaSetReconcileFails(t *testing.T) {
	psmHandler := &fakeControllerPsm{
		replicaSets: []psm.ReplicaSetInfo{{
			ReplicaSetId: "rs-1",
			Spec: psm.ReplicaSetSpec{
				Name:       "demo-web",
				Namespace:  "demo",
				Replicas:   1,
				TemplateId: "tpl-1",
			},
		}},
		templates: []psm.PodTemplateInfo{{TemplateId: "tpl-1"}},
		pods:      map[string]psm.PodInfo{},
	}
	podHandler := &fakeControllerPodService{
		createPodId: "pod-new",
		startErr:    errors.New("start failed"),
	}
	controller := &PodController{
		psmHandler:       psmHandler,
		podHandler:       podHandler,
		containerHandler: &fakeControllerContainerService{},
		interval:         time.Second,
	}

	if err := controller.reconcileOnce(); err != nil {
		t.Fatalf("reconcileOnce returned error: %v", err)
	}

	if psmHandler.reconcileAttempts["rs-1"] != 1 {
		t.Fatalf("expected reconcile attempt to be recorded, got %d", psmHandler.reconcileAttempts["rs-1"])
	}
	if psmHandler.reconcileErrors["rs-1"] != "start failed" {
		t.Fatalf("expected reconcile error to be recorded, got %q", psmHandler.reconcileErrors["rs-1"])
	}
}

type fakeControllerPsm struct {
	pods        map[string]psm.PodInfo
	templates   []psm.PodTemplateInfo
	replicaSets []psm.ReplicaSetInfo
	deployments []psm.DeploymentInfo
	removedPods map[string]bool
	owners      map[string]struct {
		kind string
		id   string
	}
	reconcileAttempts map[string]int
	reconcileErrors   map[string]string
}

func (f *fakeControllerPsm) StorePod(psm.StorePodRequest) error                 { return nil }
func (f *fakeControllerPsm) StorePodTemplate(string, psm.PodTemplateSpec) error { return nil }
func (f *fakeControllerPsm) GetPodTemplate(string) (psm.PodTemplateInfo, error) {
	return psm.PodTemplateInfo{}, nil
}
func (f *fakeControllerPsm) GetPodTemplateList() ([]psm.PodTemplateInfo, error) {
	return f.templates, nil
}
func (f *fakeControllerPsm) AddContainerToPodTemplate(string, psm.ContainerTemplateSpec) error {
	return nil
}
func (f *fakeControllerPsm) RemovePodTemplate(string) error                   { return nil }
func (f *fakeControllerPsm) StoreReplicaSet(string, psm.ReplicaSetSpec) error { return nil }
func (f *fakeControllerPsm) GetReplicaSet(string) (psm.ReplicaSetInfo, error) {
	return psm.ReplicaSetInfo{}, nil
}
func (f *fakeControllerPsm) GetReplicaSetList() ([]psm.ReplicaSetInfo, error) {
	return f.replicaSets, nil
}
func (f *fakeControllerPsm) IsTemplateReferenced(string) (bool, error)  { return true, nil }
func (f *fakeControllerPsm) UpdateReplicaSetReplicas(string, int) error { return nil }
func (f *fakeControllerPsm) UpdateReplicaSetReconcileStatus(replicaSetId string, attempt int, lastError string, nextReconcileAt time.Time) error {
	if f.reconcileAttempts == nil {
		f.reconcileAttempts = make(map[string]int)
	}
	if f.reconcileErrors == nil {
		f.reconcileErrors = make(map[string]string)
	}
	f.reconcileAttempts[replicaSetId] = attempt
	f.reconcileErrors[replicaSetId] = lastError
	return nil
}
func (f *fakeControllerPsm) ClearReplicaSetReconcileStatus(replicaSetId string) error {
	if f.reconcileAttempts != nil {
		delete(f.reconcileAttempts, replicaSetId)
	}
	if f.reconcileErrors != nil {
		delete(f.reconcileErrors, replicaSetId)
	}
	return nil
}
func (f *fakeControllerPsm) RemoveReplicaSet(string) error                    { return nil }
func (f *fakeControllerPsm) StoreDeployment(string, psm.DeploymentSpec) error { return nil }
func (f *fakeControllerPsm) GetDeployment(string) (psm.DeploymentInfo, error) {
	return psm.DeploymentInfo{}, nil
}
func (f *fakeControllerPsm) GetDeploymentList() ([]psm.DeploymentInfo, error) {
	return f.deployments, nil
}
func (f *fakeControllerPsm) UpdateDeploymentReplicas(string, int) error      { return nil }
func (f *fakeControllerPsm) UpdateDeploymentReplicaSet(string, string) error { return nil }
func (f *fakeControllerPsm) RemoveDeployment(string) error                   { return nil }
func (f *fakeControllerPsm) RemovePod(podId string) error {
	if _, ok := f.pods[podId]; !ok {
		return fmt.Errorf("podId=%s not found", podId)
	}
	if f.removedPods == nil {
		f.removedPods = make(map[string]bool)
	}
	f.removedPods[podId] = true
	delete(f.pods, podId)
	return nil
}
func (f *fakeControllerPsm) UpdatePod(string, string) error { return nil }
func (f *fakeControllerPsm) UpdatePodOwner(podId, ownerKind, ownerId string) error {
	if f.owners == nil {
		f.owners = make(map[string]struct {
			kind string
			id   string
		})
	}
	f.owners[podId] = struct {
		kind string
		id   string
	}{kind: ownerKind, id: ownerId}
	return nil
}
func (f *fakeControllerPsm) UpdatePodStoppedByUser(string, bool) error { return nil }
func (f *fakeControllerPsm) UpdatePodNamespaces(int, string, string, string, string, string) error {
	return nil
}
func (f *fakeControllerPsm) ResetPodNamespaces(string) error { return nil }
func (f *fakeControllerPsm) GetPodList() ([]psm.PodInfo, error) {
	pods := make([]psm.PodInfo, 0, len(f.pods))
	for _, p := range f.pods {
		pods = append(pods, p)
	}
	return pods, nil
}
func (f *fakeControllerPsm) GetPodById(podId string) (psm.PodInfo, error) {
	p, ok := f.pods[podId]
	if !ok {
		return psm.PodInfo{}, fmt.Errorf("pod: %s not found", podId)
	}
	return p, nil
}
func (f *fakeControllerPsm) IsNameAlreadyUsed(string, string) bool { return false }
func (f *fakeControllerPsm) GetPodIdByName(string, string) (string, error) {
	return "", fmt.Errorf("not found")
}
func (f *fakeControllerPsm) ResolvePodId(string, string) (string, error) { return "", nil }
func (f *fakeControllerPsm) IsPodExist(string) bool                      { return true }
func (f *fakeControllerPsm) IsPodOwner(string) bool                      { return false }
func (f *fakeControllerPsm) GetPodOwnerPid(string) (int, error)          { return 0, nil }

type fakeControllerPodService struct {
	createPodId         string
	createdTemplateId   string
	createdName         string
	recreatePodId       string
	recreatedTemplateId string
	startedPodId        string
	startErr            error
}

func (f *fakeControllerPodService) Create(ServiceCreateModel) (string, error) { return "", nil }
func (f *fakeControllerPodService) RecreateFromTemplate(templateId string) (string, error) {
	f.recreatedTemplateId = templateId
	return f.recreatePodId, nil
}
func (f *fakeControllerPodService) CreateFromTemplate(templateId, name string) (string, error) {
	f.createdTemplateId = templateId
	f.createdName = name
	return f.createPodId, nil
}
func (f *fakeControllerPodService) Start(podId string) (string, error) {
	if f.startErr != nil {
		return "", f.startErr
	}
	f.startedPodId = podId
	return podId, nil
}
func (f *fakeControllerPodService) Stop(string) (string, error)         { return "", nil }
func (f *fakeControllerPodService) Remove(string) (string, error)       { return "", nil }
func (f *fakeControllerPodService) GetPodList() ([]PodState, error)     { return nil, nil }
func (f *fakeControllerPodService) GetPodById(string) (PodState, error) { return PodState{}, nil }

type fakeControllerContainerService struct {
	containersByPod map[string][]container.ContainerState
	stopped         map[string]bool
	deleted         map[string]bool
}

func (f *fakeControllerContainerService) Create(container.ServiceCreateModel) (string, error) {
	return "", nil
}
func (f *fakeControllerContainerService) Start(container.ServiceStartModel) (string, error) {
	return "", nil
}
func (f *fakeControllerContainerService) Delete(param container.ServiceDeleteModel) (string, error) {
	if f.deleted == nil {
		f.deleted = make(map[string]bool)
	}
	f.deleted[param.ContainerId] = true
	return param.ContainerId, nil
}
func (f *fakeControllerContainerService) Stop(param container.ServiceStopModel) (string, error) {
	if f.stopped == nil {
		f.stopped = make(map[string]bool)
	}
	f.stopped[param.ContainerId] = true
	return param.ContainerId, nil
}
func (f *fakeControllerContainerService) Exec(container.ServiceExecModel) error { return nil }
func (f *fakeControllerContainerService) GetContainerList() ([]container.ContainerState, error) {
	return nil, nil
}
func (f *fakeControllerContainerService) GetContainerById(string) (container.ContainerState, error) {
	return container.ContainerState{}, nil
}
func (f *fakeControllerContainerService) GetContainerStats(string) (container.ContainerStats, error) {
	return container.ContainerStats{}, nil
}
func (f *fakeControllerContainerService) ListContainerStats() ([]container.ContainerStats, error) {
	return nil, nil
}
func (f *fakeControllerContainerService) GetContainersByPodId(podId string) ([]container.ContainerState, error) {
	return f.containersByPod[podId], nil
}
func (f *fakeControllerContainerService) GetContainerLogPath(string) (string, error) { return "", nil }
func (f *fakeControllerContainerService) GetContainerSpec(string) (map[string]any, error) {
	return nil, nil
}
func (f *fakeControllerContainerService) InspectContainer(string) (container.ContainerInspect, error) {
	return container.ContainerInspect{}, nil
}
func (f *fakeControllerContainerService) GetLogWithTailLines(string, int) ([]byte, error) {
	return nil, nil
}
