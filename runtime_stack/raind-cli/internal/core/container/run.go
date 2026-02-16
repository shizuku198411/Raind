package container

import "time"

func NewServiceContainerRun() *ServiceContianerRun {
	return &ServiceContianerRun{}
}

type ServiceContianerRun struct{}

func (s *ServiceContianerRun) Run(param ServiceRunModel) error {
	// 1. create
	serviceCreate := NewServiceContainerCreate()
	containerId, err := serviceCreate.Create(
		ServiceCreateModel{
			Image:   param.Image,
			Command: param.Command,
			Network: param.Network,
			Volume:  param.Volume,
			Publish: param.Publish,
			Env:     param.Env,
			Tty:     param.Tty,
			Name:    param.Name,
		},
	)
	if err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond)

	// 2. start
	serviceStart := NewServiceContainerStart()
	if err := serviceStart.Start(
		ServiceStartModel{
			Id:  containerId,
			Tty: param.Tty,
		},
	); err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond)

	// 3. attach
	if param.Tty {
		serviceAttach := NewServiceContainerAttach()
		if err := serviceAttach.Attach(containerId); err != nil {
			return err
		}
	}

	// 4. remove
	time.Sleep(200 * time.Millisecond) // wait for hook
	if param.Rm {
		serviceRemove := NewServiceContainerRemove()
		if err := serviceRemove.Remove(ServiceRemoveModel{Id: containerId}); err != nil {
			return err
		}
	}

	return nil
}
