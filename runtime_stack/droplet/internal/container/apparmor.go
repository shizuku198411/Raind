package container

import (
	"droplet/internal/utils"
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
)

type AppArmorHandler interface {
	ApplyAAProfile(profile string) error
	ApplyAAProfileOnExec(profile string) error
}

func NewAppArmorManager() *AppArmorManager {
	return &AppArmorManager{
		syscallHandler: utils.NewSyscallHandler(),
	}
}

type AppArmorManager struct {
	syscallHandler utils.KernelSyscallHandler
}

// ApplyAAProfile switches the current task to the given AppArmor profile.
//
// Must be called in the container init process, as late as possible (typically
// right before execve), because the profile applies to the current task.
//
// Notes:
// - Requires AppArmor enabled on the host.
// - The profile must be loaded in the host kernel beforehand (e.g., via apparmor_parser).
func (m *AppArmorManager) ApplyAAProfile(profile string) error {
	// check if AppArmor is enabled on host
	if !m.isAAEnabled() {
		return nil
	}

	profile = strings.TrimSpace(profile)
	if profile == "" {
		return nil
	}

	// AppArmor procfs interface:
	//   /proc/self/attr/current
	// write format:
	//   "changeprofile <profile>"
	const aaAttrCurrent = "/proc/self/attr/current"

	f, err := m.syscallHandler.OpenFile(aaAttrCurrent, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open %s failed (is AppArmor enabled?): %w", aaAttrCurrent, err)
	}

	cmd := "changeprofile " + profile
	if _, err := f.WriteString(cmd); err != nil {
		return fmt.Errorf("AppArmor changeprofile to %q failed: %w", profile, err)
	}

	return nil
}

func (m *AppArmorManager) ApplyAAProfileOnExec(profile string) error {
	if !m.isAAEnabled() {
		return nil
	}

	profile = strings.TrimSpace(profile)
	if profile == "" {
		return nil
	}

	// AppArmor procfs interface for onexec:
	// /proc/self/attr/exec
	const aaAttrExec = "/proc/self/attr/exec"

	f, err := m.syscallHandler.OpenFile(aaAttrExec, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open %s failed: %w", aaAttrExec, err)
	}
	defer f.Close()

	cmd := "exec " + profile
	if _, err := f.WriteString(cmd); err != nil {
		if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
			return nil
		}
		return fmt.Errorf("AppArmor exec profile %q failed: %w", profile, err)
	}
	return nil
}

func (m *AppArmorManager) isAAEnabled() bool {
	_, err := m.syscallHandler.Stat("/sys/module/apparmor/parameters/enabled")
	return err == nil
}
