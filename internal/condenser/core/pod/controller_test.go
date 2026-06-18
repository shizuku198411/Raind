package pod

import (
	"fmt"
	"testing"

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

type fakeControllerPsm struct {
	pods        map[string]psm.PodInfo
	templates   []psm.PodTemplateInfo
	replicaSets []psm.ReplicaSetInfo
	deployments []psm.DeploymentInfo
	removedPods map[string]bool
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
func (f *fakeControllerPsm) IsTemplateReferenced(string) (bool, error)        { return true, nil }
func (f *fakeControllerPsm) UpdateReplicaSetReplicas(string, int) error       { return nil }
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
func (f *fakeControllerPsm) UpdatePod(string, string) error            { return nil }
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
	recreatePodId       string
	recreatedTemplateId string
	startedPodId        string
}

func (f *fakeControllerPodService) Create(ServiceCreateModel) (string, error) { return "", nil }
func (f *fakeControllerPodService) RecreateFromTemplate(templateId string) (string, error) {
	f.recreatedTemplateId = templateId
	return f.recreatePodId, nil
}
func (f *fakeControllerPodService) CreateFromTemplate(string, string) (string, error) { return "", nil }
func (f *fakeControllerPodService) Start(podId string) (string, error) {
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
