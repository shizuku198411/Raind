package vsm

import (
	"fmt"
	"time"
)

func NewManager(store *Store) *Manager {
	return &Manager{store: store}
}

type Manager struct {
	store *Store
}

func (m *Manager) StorePVC(pvcId string, info PersistentVolumeClaimInfo) error {
	return m.store.withLock(func(st *VolumeState) error {
		info.PVCId = pvcId
		if info.Namespace == "" {
			info.Namespace = "default"
		}
		if info.ReclaimPolicy == "" {
			info.ReclaimPolicy = PVCReclaimRetain
		}
		if info.Phase == "" {
			info.Phase = PVCPhaseBound
		}
		if info.CreatedAt.IsZero() {
			info.CreatedAt = time.Now()
		}
		st.PVCs[pvcId] = info
		return nil
	})
}

func (m *Manager) GetPVCList() ([]PersistentVolumeClaimInfo, error) {
	var list []PersistentVolumeClaimInfo
	err := m.store.withRLock(func(st *VolumeState) error {
		for _, pvc := range st.PVCs {
			list = append(list, pvc)
		}
		return nil
	})
	return list, err
}

func (m *Manager) GetPVCById(pvcId string) (PersistentVolumeClaimInfo, error) {
	var info PersistentVolumeClaimInfo
	err := m.store.withRLock(func(st *VolumeState) error {
		pvc, ok := st.PVCs[pvcId]
		if !ok {
			return fmt.Errorf("pvcId=%s not found", pvcId)
		}
		info = pvc
		return nil
	})
	return info, err
}

func (m *Manager) GetPVCByName(name, namespace string) (PersistentVolumeClaimInfo, error) {
	if namespace == "" {
		namespace = "default"
	}
	var info PersistentVolumeClaimInfo
	err := m.store.withRLock(func(st *VolumeState) error {
		for _, pvc := range st.PVCs {
			if pvc.Name == name && pvc.Namespace == namespace {
				info = pvc
				return nil
			}
		}
		return fmt.Errorf("pvc %s/%s not found", namespace, name)
	})
	return info, err
}

func (m *Manager) RemovePVC(pvcId string) error {
	return m.store.withLock(func(st *VolumeState) error {
		if _, ok := st.PVCs[pvcId]; !ok {
			return fmt.Errorf("pvcId=%s not found", pvcId)
		}
		delete(st.PVCs, pvcId)
		return nil
	})
}

func (m *Manager) IsNameAlreadyUsed(name, namespace string) bool {
	if namespace == "" {
		namespace = "default"
	}
	var used bool
	_ = m.store.withRLock(func(st *VolumeState) error {
		for _, pvc := range st.PVCs {
			if pvc.Name == name && pvc.Namespace == namespace {
				used = true
				return nil
			}
		}
		return nil
	})
	return used
}
