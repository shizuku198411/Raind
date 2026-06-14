package ism

import (
	"fmt"
	"time"
)

func NewIsmManager(ismStore *IsmStore) *IsmManager {
	return &IsmManager{ismStore: ismStore}
}

type IsmManager struct {
	ismStore *IsmStore
}

func (m *IsmManager) StoreIngress(ingressId string, spec IngressInfo) error {
	return m.ismStore.withLock(func(st *IngressState) error {
		spec.IngressId = ingressId
		spec.CreatedAt = time.Now()
		st.Ingresses[ingressId] = spec
		return nil
	})
}

func (m *IsmManager) GetIngressList() ([]IngressInfo, error) {
	var list []IngressInfo
	err := m.ismStore.withRLock(func(st *IngressState) error {
		for _, in := range st.Ingresses {
			list = append(list, in)
		}
		return nil
	})
	return list, err
}

func (m *IsmManager) GetIngressById(ingressId string) (IngressInfo, error) {
	var info IngressInfo
	err := m.ismStore.withRLock(func(st *IngressState) error {
		in, ok := st.Ingresses[ingressId]
		if !ok {
			return fmt.Errorf("ingressId=%s not found", ingressId)
		}
		info = in
		return nil
	})
	return info, err
}

func (m *IsmManager) RemoveIngress(ingressId string) error {
	return m.ismStore.withLock(func(st *IngressState) error {
		if _, ok := st.Ingresses[ingressId]; !ok {
			return fmt.Errorf("ingressId=%s not found", ingressId)
		}
		delete(st.Ingresses, ingressId)
		return nil
	})
}

func (m *IsmManager) IsNameAlreadyUsed(name, namespace string) bool {
	var used bool
	_ = m.ismStore.withRLock(func(st *IngressState) error {
		for _, in := range st.Ingresses {
			if in.Name == name && in.Namespace == namespace {
				used = true
				return nil
			}
		}
		return nil
	})
	return used
}
