package cfm

type CfmHandler interface {
	StoreConfigMap(configMapId string, spec ConfigMapInfo) error
	GetConfigMapList() ([]ConfigMapInfo, error)
	GetConfigMapById(configMapId string) (ConfigMapInfo, error)
	GetConfigMapByName(name, namespace string) (ConfigMapInfo, error)
	RemoveConfigMap(configMapId string) error
	IsNameAlreadyUsed(name, namespace string) bool
}
