package container

import (
	"raind/internal/droplet/container/security"
	"raind/internal/droplet/utils"
)

type AppArmorHandler interface {
	ApplyAAProfile(profile string) error
	ApplyAAProfileOnExec(profile string) error
}

type AppArmorManager struct {
	syscallHandler utils.KernelSyscallHandler
}

func NewAppArmorManager() *AppArmorManager {
	return &AppArmorManager{syscallHandler: utils.NewSyscallHandler()}
}

func (m *AppArmorManager) securityManager() *security.AppArmorManager {
	return &security.AppArmorManager{SyscallHandler: m.syscallHandler}
}

func (m *AppArmorManager) ApplyAAProfile(profile string) error {
	return m.securityManager().ApplyAAProfile(profile)
}

func (m *AppArmorManager) ApplyAAProfileOnExec(profile string) error {
	return m.securityManager().ApplyAAProfileOnExec(profile)
}

func (m *AppArmorManager) isAAEnabled() bool {
	return m.securityManager().IsAAEnabled()
}
