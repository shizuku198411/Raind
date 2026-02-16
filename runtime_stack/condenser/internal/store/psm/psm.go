package psm

import (
	"fmt"
	"time"
)

func NewPsmManager(psmStore *PsmStore) *PsmManager {
	return &PsmManager{
		psmStore: psmStore,
	}
}

type PsmManager struct {
	psmStore *PsmStore
}

func (m *PsmManager) StorePod(podId, templateId, name, namespace, uid, state, networkNS, ipcNS, utsNS, userNS string, labels, annotations map[string]string) error {
	return m.psmStore.withLock(func(st *PodState) error {
		st.Pods[podId] = PodInfo{
			PodId:       podId,
			TemplateId:  templateId,
			Name:        name,
			Namespace:   namespace,
			UID:         uid,
			State:       state,
			NetworkNS:   networkNS,
			IPCNS:       ipcNS,
			UTSNS:       utsNS,
			UserNS:      userNS,
			Labels:      labels,
			Annotations: annotations,
			CreatedAt:   time.Now(),
		}
		return nil
	})
}

func (m *PsmManager) StorePodTemplate(templateId string, spec PodTemplateSpec) error {
	return m.psmStore.withLock(func(st *PodState) error {
		st.PodTemplates[templateId] = PodTemplateInfo{
			TemplateId: templateId,
			Spec:       spec,
			CreatedAt:  time.Now(),
		}
		return nil
	})
}

func (m *PsmManager) GetPodTemplate(templateId string) (PodTemplateInfo, error) {
	var template PodTemplateInfo
	err := m.psmStore.withRLock(func(st *PodState) error {
		t, ok := st.PodTemplates[templateId]
		if !ok {
			return fmt.Errorf("podTemplateId=%s not found", templateId)
		}
		template = t
		return nil
	})
	return template, err
}

func (m *PsmManager) GetPodTemplateList() ([]PodTemplateInfo, error) {
	var templates []PodTemplateInfo
	err := m.psmStore.withRLock(func(st *PodState) error {
		for _, t := range st.PodTemplates {
			templates = append(templates, t)
		}
		return nil
	})
	return templates, err
}

func (m *PsmManager) AddContainerToPodTemplate(podId string, spec ContainerTemplateSpec) error {
	return m.psmStore.withLock(func(st *PodState) error {
		pod, ok := st.Pods[podId]
		if !ok {
			return fmt.Errorf("podId=%s not found", podId)
		}
		if pod.TemplateId == "" {
			return fmt.Errorf("podId=%s has no templateId", podId)
		}
		tpl, ok := st.PodTemplates[pod.TemplateId]
		if !ok {
			return fmt.Errorf("podTemplateId=%s not found", pod.TemplateId)
		}

		replaced := false
		for i, c := range tpl.Spec.Containers {
			if c.Name == spec.Name && spec.Name != "" {
				tpl.Spec.Containers[i] = spec
				replaced = true
				break
			}
		}
		if !replaced {
			tpl.Spec.Containers = append(tpl.Spec.Containers, spec)
		}

		st.PodTemplates[pod.TemplateId] = tpl
		return nil
	})
}

func (m *PsmManager) RemovePodTemplate(templateId string) error {
	return m.psmStore.withLock(func(st *PodState) error {
		if _, ok := st.PodTemplates[templateId]; !ok {
			return fmt.Errorf("podTemplateId=%s not found", templateId)
		}
		delete(st.PodTemplates, templateId)
		return nil
	})
}

func (m *PsmManager) StoreReplicaSet(replicaSetId string, spec ReplicaSetSpec) error {
	return m.psmStore.withLock(func(st *PodState) error {
		st.ReplicaSets[replicaSetId] = ReplicaSetInfo{
			ReplicaSetId: replicaSetId,
			Spec:         spec,
			CreatedAt:    time.Now(),
		}
		return nil
	})
}

func (m *PsmManager) GetReplicaSet(replicaSetId string) (ReplicaSetInfo, error) {
	var rs ReplicaSetInfo
	err := m.psmStore.withRLock(func(st *PodState) error {
		info, ok := st.ReplicaSets[replicaSetId]
		if !ok {
			return fmt.Errorf("replicaSetId=%s not found", replicaSetId)
		}
		rs = info
		return nil
	})
	return rs, err
}

func (m *PsmManager) GetReplicaSetList() ([]ReplicaSetInfo, error) {
	var sets []ReplicaSetInfo
	err := m.psmStore.withRLock(func(st *PodState) error {
		for _, rs := range st.ReplicaSets {
			sets = append(sets, rs)
		}
		return nil
	})
	return sets, err
}

func (m *PsmManager) UpdateReplicaSetReplicas(replicaSetId string, replicas int) error {
	return m.psmStore.withLock(func(st *PodState) error {
		rs, ok := st.ReplicaSets[replicaSetId]
		if !ok {
			return fmt.Errorf("replicaSetId=%s not found", replicaSetId)
		}
		if replicas < 0 {
			return fmt.Errorf("replicas must be >= 0")
		}
		rs.Spec.Replicas = replicas
		st.ReplicaSets[replicaSetId] = rs
		return nil
	})
}

func (m *PsmManager) IsTemplateReferenced(templateId string) (bool, error) {
	var referenced bool
	err := m.psmStore.withRLock(func(st *PodState) error {
		for _, rs := range st.ReplicaSets {
			if rs.Spec.TemplateId == templateId {
				referenced = true
				return nil
			}
		}
		return nil
	})
	return referenced, err
}

func (m *PsmManager) RemoveReplicaSet(replicaSetId string) error {
	return m.psmStore.withLock(func(st *PodState) error {
		if _, ok := st.ReplicaSets[replicaSetId]; !ok {
			return fmt.Errorf("replicaSetId=%s not found", replicaSetId)
		}
		delete(st.ReplicaSets, replicaSetId)
		return nil
	})
}

func (m *PsmManager) RemovePod(podId string) error {
	return m.psmStore.withLock(func(st *PodState) error {
		if _, ok := st.Pods[podId]; !ok {
			return fmt.Errorf("podId=%s not found", podId)
		}
		delete(st.Pods, podId)
		return nil
	})
}

func (m *PsmManager) UpdatePod(podId string, state string) error {
	return m.psmStore.withLock(func(st *PodState) error {
		p, ok := st.Pods[podId]
		if !ok {
			return fmt.Errorf("podId=%s not found", podId)
		}

		p.State = state
		switch state {
		case "created":
			p.CreatedAt = time.Now()
		case "running":
			p.StartedAt = time.Now()
		case "stopped":
			p.StoppedAt = time.Now()
		}

		st.Pods[podId] = p
		return nil
	})
}

func (m *PsmManager) UpdatePodStoppedByUser(podId string, stopped bool) error {
	return m.psmStore.withLock(func(st *PodState) error {
		p, ok := st.Pods[podId]
		if !ok {
			return fmt.Errorf("podId=%s not found", podId)
		}
		p.StoppedByUser = stopped
		st.Pods[podId] = p
		return nil
	})
}

func (m *PsmManager) UpdatePodNamespaces(ownerPid int, podId, networkNS, ipcNS, utsNS, userNS string) error {
	return m.psmStore.withLock(func(st *PodState) error {
		p, ok := st.Pods[podId]
		if !ok {
			return fmt.Errorf("podId=%s not found", podId)
		}

		p.OwnerPid = ownerPid

		if p.NetworkNS == "" && networkNS != "" {
			p.NetworkNS = networkNS
		}
		if p.IPCNS == "" && ipcNS != "" {
			p.IPCNS = ipcNS
		}
		if p.UTSNS == "" && utsNS != "" {
			p.UTSNS = utsNS
		}
		if p.UserNS == "" && userNS != "" {
			p.UserNS = userNS
		}

		st.Pods[podId] = p
		return nil
	})
}

func (m *PsmManager) ResetPodNamespaces(podId string) error {
	return m.psmStore.withLock(func(st *PodState) error {
		p, ok := st.Pods[podId]
		if !ok {
			return fmt.Errorf("podId=%s not found", podId)
		}
		p.OwnerPid = 0
		p.NetworkNS = ""
		p.IPCNS = ""
		p.UTSNS = ""
		p.UserNS = ""
		st.Pods[podId] = p
		return nil
	})
}

func (m *PsmManager) GetPodList() ([]PodInfo, error) {
	var podList []PodInfo
	err := m.psmStore.withRLock(func(st *PodState) error {
		for _, p := range st.Pods {
			podList = append(podList, p)
		}
		return nil
	})
	return podList, err
}

func (m *PsmManager) GetPodById(podId string) (PodInfo, error) {
	var podInfo PodInfo
	err := m.psmStore.withRLock(func(st *PodState) error {
		p, ok := st.Pods[podId]
		if !ok {
			return fmt.Errorf("pod: %s not found", podId)
		}
		podInfo = p
		return nil
	})
	return podInfo, err
}

func (m *PsmManager) IsNameAlreadyUsed(name, namespace string) bool {
	var result bool
	_ = m.psmStore.withRLock(func(st *PodState) error {
		for _, p := range st.Pods {
			if p.Name == name && p.Namespace == namespace {
				result = true
				return nil
			}
		}
		result = false
		return nil
	})
	return result
}

func (m *PsmManager) GetPodIdByName(name, namespace string) (string, error) {
	var podId string
	err := m.psmStore.withRLock(func(st *PodState) error {
		for _, p := range st.Pods {
			if p.Name == name && p.Namespace == namespace {
				podId = p.PodId
				return nil
			}
		}
		return fmt.Errorf("pod: %s/%s not found", namespace, name)
	})
	return podId, err
}

func (m *PsmManager) ResolvePodId(str, namespace string) (string, error) {
	if namespace != "" {
		if podId, err := m.GetPodIdByName(str, namespace); err == nil {
			return podId, nil
		}
	}
	if _, err := m.GetPodById(str); err != nil {
		return "", err
	}
	return str, nil
}

func (m *PsmManager) IsPodExist(podId string) bool {
	_, err := m.GetPodById(podId)
	return err == nil
}

func (m *PsmManager) IsPodOwner(podId string) bool {
	if podId == "" {
		return false
	}
	var result bool
	_ = m.psmStore.withRLock(func(st *PodState) error {
		p, ok := st.Pods[podId]
		if !ok {
			return fmt.Errorf("pod: %s not found", podId)
		}
		if p.UserNS == "" && p.NetworkNS == "" && p.IPCNS == "" && p.UTSNS == "" {
			result = true
			return nil
		}
		result = false
		return nil
	})
	return result
}

func (m *PsmManager) GetPodOwnerPid(podId string) (int, error) {
	var ownerPid int
	err := m.psmStore.withRLock(func(st *PodState) error {
		p, ok := st.Pods[podId]
		if !ok {
			return fmt.Errorf("pod: %s not found", podId)
		}
		ownerPid = p.OwnerPid
		return nil
	})
	return ownerPid, err
}
