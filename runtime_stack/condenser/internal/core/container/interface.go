package container

type ContainerServiceHandler interface {
	Create(createParameter ServiceCreateModel) (string, error)
	Start(startParameter ServiceStartModel) (string, error)
	Delete(deleteParameter ServiceDeleteModel) (string, error)
	Stop(stopParameter ServiceStopModel) (string, error)
	Exec(execParameter ServiceExecModel) error
	GetContainerList() ([]ContainerState, error)
	GetContainerById(containerId string) (ContainerState, error)
	GetContainerStats(containerId string) (ContainerStats, error)
	ListContainerStats() ([]ContainerStats, error)
	GetContainersByPodId(podId string) ([]ContainerState, error)
	GetContainerLogPath(target string) (string, error)
	GetLogWithTailLines(containerId string, n int) ([]byte, error)
}

type CgroupServiceHandler interface {
	ChangeCgroupMode(containerId string) error
}
