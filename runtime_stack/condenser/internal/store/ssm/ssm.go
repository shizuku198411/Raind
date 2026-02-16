package ssm

import (
	"fmt"
	"time"
)

func NewSsmManager(ssmStore *SsmStore) *SsmManager {
	return &SsmManager{
		ssmStore: ssmStore,
	}
}

type SsmManager struct {
	ssmStore *SsmStore
}

func (m *SsmManager) StoreService(serviceId string, spec ServiceInfo) error {
	return m.ssmStore.withLock(func(st *ServiceState) error {
		spec.ServiceId = serviceId
		spec.CreatedAt = time.Now()
		st.Services[serviceId] = spec
		return nil
	})
}

func (m *SsmManager) GetServiceList() ([]ServiceInfo, error) {
	var list []ServiceInfo
	err := m.ssmStore.withRLock(func(st *ServiceState) error {
		for _, s := range st.Services {
			list = append(list, s)
		}
		return nil
	})
	return list, err
}

func (m *SsmManager) GetServiceById(serviceId string) (ServiceInfo, error) {
	var info ServiceInfo
	err := m.ssmStore.withRLock(func(st *ServiceState) error {
		s, ok := st.Services[serviceId]
		if !ok {
			return fmt.Errorf("serviceId=%s not found", serviceId)
		}
		info = s
		return nil
	})
	return info, err
}

func (m *SsmManager) RemoveService(serviceId string) error {
	return m.ssmStore.withLock(func(st *ServiceState) error {
		if _, ok := st.Services[serviceId]; !ok {
			return fmt.Errorf("serviceId=%s not found", serviceId)
		}
		delete(st.Services, serviceId)
		return nil
	})
}

func (m *SsmManager) IsNameAlreadyUsed(name, namespace string) bool {
	var used bool
	_ = m.ssmStore.withRLock(func(st *ServiceState) error {
		for _, s := range st.Services {
			if s.Name == name && s.Namespace == namespace {
				used = true
				return nil
			}
		}
		return nil
	})
	return used
}
