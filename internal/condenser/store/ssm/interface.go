package ssm

type SsmHandler interface {
	StoreService(serviceId string, spec ServiceInfo) error
	GetServiceList() ([]ServiceInfo, error)
	GetServiceById(serviceId string) (ServiceInfo, error)
	RemoveService(serviceId string) error
	IsNameAlreadyUsed(name, namespace string) bool
}
