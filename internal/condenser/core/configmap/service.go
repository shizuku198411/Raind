package configmap

import (
	"fmt"

	"raind/internal/condenser/store/cfm"
	"raind/internal/condenser/utils"
)

func NewConfigMapService() *ConfigMapService {
	return &ConfigMapService{
		cfmHandler: cfm.NewCfmManager(cfm.NewCfmStore(utils.CfmStorePath)),
	}
}

type ConfigMapService struct {
	cfmHandler cfm.CfmHandler
}

func (s *ConfigMapService) Create(manifest Manifest) (cfm.ConfigMapInfo, error) {
	if manifest.Name == "" {
		return cfm.ConfigMapInfo{}, fmt.Errorf("configmap name is required")
	}
	if manifest.Namespace == "" {
		manifest.Namespace = "default"
	}
	if s.cfmHandler.IsNameAlreadyUsed(manifest.Name, manifest.Namespace) {
		return cfm.ConfigMapInfo{}, fmt.Errorf("name already used by other configmap")
	}
	configMapId := utils.NewUlid()
	info := cfm.ConfigMapInfo{
		Name:      manifest.Name,
		Namespace: manifest.Namespace,
		Data:      manifest.Data,
	}
	if err := s.cfmHandler.StoreConfigMap(configMapId, info); err != nil {
		return cfm.ConfigMapInfo{}, err
	}
	return s.cfmHandler.GetConfigMapById(configMapId)
}

func (s *ConfigMapService) List(namespace string) ([]cfm.ConfigMapInfo, error) {
	list, err := s.cfmHandler.GetConfigMapList()
	if err != nil {
		return nil, err
	}
	if namespace == "" {
		return list, nil
	}
	out := make([]cfm.ConfigMapInfo, 0, len(list))
	for _, cm := range list {
		if cm.Namespace == namespace {
			out = append(out, cm)
		}
	}
	return out, nil
}

func (s *ConfigMapService) Get(idOrName string, namespace string) (cfm.ConfigMapInfo, error) {
	if namespace != "" {
		return s.cfmHandler.GetConfigMapByName(idOrName, namespace)
	}
	if info, err := s.cfmHandler.GetConfigMapById(idOrName); err == nil {
		return info, nil
	}
	list, err := s.cfmHandler.GetConfigMapList()
	if err != nil {
		return cfm.ConfigMapInfo{}, err
	}
	var found []cfm.ConfigMapInfo
	for _, cm := range list {
		if cm.Name == idOrName {
			found = append(found, cm)
		}
	}
	if len(found) == 1 {
		return found[0], nil
	}
	if len(found) > 1 {
		return cfm.ConfigMapInfo{}, fmt.Errorf("configmap name %q exists in multiple namespaces; specify namespace", idOrName)
	}
	return cfm.ConfigMapInfo{}, fmt.Errorf("configmap %q not found", idOrName)
}

func (s *ConfigMapService) Remove(idOrName string, namespace string) (cfm.ConfigMapInfo, error) {
	info, err := s.Get(idOrName, namespace)
	if err != nil {
		return cfm.ConfigMapInfo{}, err
	}
	if err := s.cfmHandler.RemoveConfigMap(info.ConfigMapId); err != nil {
		return cfm.ConfigMapInfo{}, err
	}
	return info, nil
}
