package container

type ServiceLogsModel struct {
	ContainerId string
	TailLine    int
	Pager       bool
}

type LogsResponseModel struct {
	Status  string
	Message string
}
