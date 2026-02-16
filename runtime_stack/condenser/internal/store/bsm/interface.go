package bsm

type BsmStoreHandler interface {
	SetBottleState() error
}

type BsmHandler interface {
	StoreBottle(bottleId string, bottleName string, services map[string]ServiceSpec, startOrder []string, policies []PolicyInfo) error
	RemoveBottle(bottleId string) error
	UpdateBottleContainers(bottleId string, containers map[string]string) error
	UpdateBottleContainer(bottleId string, serviceName string, containerId string) error
	UpdateBottleNetwork(bottleId string, network string, auto bool) error
	GetBottleList() ([]BottleInfo, error)
	GetBottleById(bottleId string) (BottleInfo, error)
	GetBottleByName(bottleName string) (BottleInfo, error)
	IsNameAlreadyUsed(name string) bool
	GetBottleIdByName(name string) (string, error)
	GetBottleNameById(bottleId string) (string, error)
	GetBottleIdAndName(str string) (id, name string, err error)
	ResolveBottleId(str string) (string, error)
	IsBottleExist(str string) bool
}
