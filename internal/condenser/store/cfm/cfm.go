package cfm

import (
	"fmt"
	"time"
)

func NewCfmManager(cfmStore *CfmStore) *CfmManager {
	return &CfmManager{cfmStore: cfmStore}
}

type CfmManager struct {
	cfmStore *CfmStore
}

func (m *CfmManager) StoreConfigMap(configMapId string, spec ConfigMapInfo) error {
	return m.cfmStore.withLock(func(st *ConfigMapState) error {
		spec.ConfigMapId = configMapId
		spec.CreatedAt = time.Now()
		if spec.Namespace == "" {
			spec.Namespace = "default"
		}
		if spec.Data == nil {
			spec.Data = map[string]string{}
		}
		st.ConfigMap[configMapId] = spec
		return nil
	})
}

func (m *CfmManager) GetConfigMapList() ([]ConfigMapInfo, error) {
	var list []ConfigMapInfo
	err := m.cfmStore.withRLock(func(st *ConfigMapState) error {
		for _, cm := range st.ConfigMap {
			list = append(list, cm)
		}
		return nil
	})
	return list, err
}

func (m *CfmManager) GetConfigMapById(configMapId string) (ConfigMapInfo, error) {
	var info ConfigMapInfo
	err := m.cfmStore.withRLock(func(st *ConfigMapState) error {
		cm, ok := st.ConfigMap[configMapId]
		if !ok {
			return fmt.Errorf("configMapId=%s not found", configMapId)
		}
		info = cm
		return nil
	})
	return info, err
}

func (m *CfmManager) GetConfigMapByName(name, namespace string) (ConfigMapInfo, error) {
	if namespace == "" {
		namespace = "default"
	}
	var info ConfigMapInfo
	err := m.cfmStore.withRLock(func(st *ConfigMapState) error {
		for _, cm := range st.ConfigMap {
			if cm.Name == name && cm.Namespace == namespace {
				info = cm
				return nil
			}
		}
		return fmt.Errorf("configmap %s/%s not found", namespace, name)
	})
	return info, err
}

func (m *CfmManager) RemoveConfigMap(configMapId string) error {
	return m.cfmStore.withLock(func(st *ConfigMapState) error {
		if _, ok := st.ConfigMap[configMapId]; !ok {
			return fmt.Errorf("configMapId=%s not found", configMapId)
		}
		delete(st.ConfigMap, configMapId)
		return nil
	})
}

func (m *CfmManager) IsNameAlreadyUsed(name, namespace string) bool {
	if namespace == "" {
		namespace = "default"
	}
	var used bool
	_ = m.cfmStore.withRLock(func(st *ConfigMapState) error {
		for _, cm := range st.ConfigMap {
			if cm.Name == name && cm.Namespace == namespace {
				used = true
				return nil
			}
		}
		return nil
	})
	return used
}
