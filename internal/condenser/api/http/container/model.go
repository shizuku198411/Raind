package container

// == create ==
type CreateContainerRequest struct {
	Image    string   `json:"image" example:"alpine:latest"`
	Command  []string `json:"command,omitempty" example:"/bin/sh,-c,echo hello; sleep 60"`
	Port     []string `json:"port" example:"8080:80,4443:443"`
	Mount    []string `json:"mount" example:"/host/dir:/container/dir,/src:/dst"`
	Device   []string `json:"device,omitempty" example:"/dev/net/tun,/dev/kvm:/dev/kvm"`
	Env      []string `json:"env" exampe:"key=value"`
	CapAdd   []string `json:"capAdd,omitempty" example:"CAP_NET_ADMIN,CAP_SYS_TIME"`
	CapDrop  []string `json:"capDrop,omitempty" example:"CAP_NET_RAW,CAP_SYS_ADMIN"`
	Network  string   `json:"network" example:"raind0"`
	Tty      bool     `json:"tty" example:"false"`
	Name     string   `json:"name"  example:"my-container"`
	PodId    string   `json:"podId" example:"pod-1234"`
	Rootless bool     `json:"rootless" example:"false"`
}

type CreateContainerResponse struct {
	Id string `json:"id"`
}

// == start ==
type StartContainerRequest struct {
	Tty bool `json:"tty" example:"false"`
}

type StartContainerResponse struct {
	Id string `json:"id"`
}

// == stop ==
type StopContainerResponse struct {
	Id string `json:"id"`
}

// == exec ==
type ExecContainerRequest struct {
	Command []string `json:"command" example:"/bin/sh,-c,echo hello"`
	Tty     bool     `json:"tty" example:"true"`
}

type ExecContainerResponse struct {
	Id string `json:"id"`
}

// == delete ==
type DeleteContainerResponse struct {
	Id string `json:"id"`
}
