package csm

type CsmStoreHandler interface {
	SetContainerState() error
}

type CsmHandler interface {
	StoreContainer(containerId string, state string, pid int, tty bool, repo, ref string, command []string, name string, bottleId string, logPath string, podId string) error
	RemoveContainer(containerId string) error
	UpdateContainer(containerId string, state string, pid int) error
	UpdateExitStatus(containerId string, exitCode int, reason string, message string) error
	UpdateSpiffe(containerId string, spiffe string) error
	GetContainerList() ([]ContainerInfo, error)
	GetContainerById(containerId string) (ContainerInfo, error)
	GetContainersByPodId(podId string) ([]ContainerInfo, error)
	IsNameAlreadyUsed(name string) bool
	GetContainerIdByName(name string) (string, error)
	GetContainerNameById(containerId string) (string, error)
	GetContainerIdAndName(str string) (id, name string, err error)
	GetSpiffeById(containerId string) (string, error)
	ResolveContainerId(str string) (string, error)
	IsContainerExist(str string) bool
	GetLogPath(containerId string) (string, error)
}
