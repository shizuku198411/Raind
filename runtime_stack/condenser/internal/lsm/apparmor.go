package lsm

import (
	"bytes"
	"condenser/internal/utils"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// AppArmor profile: raind-default
const raindDefaultProfile = `#include <tunables/global>

profile raind-default flags=(attach_disconnected,mediate_deleted) {

  #include <abstractions/base>
  #include <abstractions/nameservice>

  # ---- Baseline permissions ----
  file,
  umount,
  signal,
  ptrace (read),
  capability,

  # ---- Network policy ----
  # Allow TCP/UDP
  network inet stream,
  network inet dgram,
  network inet6 stream,
  network inet6 dgram,

  # Allow INET(Raw) for ICMP
  network inet raw,
  network inet6 raw,

  # Deny AF_PACKET(L2)
  deny network packet,
  deny network packet raw,
  deny network packet dgram,

  # Allow Netlink
  network netlink raw,

  # ---- Dangerous capabilities (deny) ----
  # Deny Capabilities
  deny capability sys_module,
  deny capability sys_rawio,
  deny capability sys_boot,
  deny capability sys_time,
  deny capability sys_tty_config,
  deny capability syslog,
  deny capability mac_admin,
  deny capability mac_override,

  # ---- Kernel interface hardening ----
  # Deny /proc
  deny /proc/kcore r,
  deny /proc/kmem r,
  deny /proc/mem r,
  deny /proc/sys/** wklx,
  deny /proc/sysrq-trigger wklx,

  # Deny /sys
  deny /sys/** wklx,
  deny /sys/kernel/** wklx,
  deny /sys/firmware/** rwklx,
  deny /sys/fs/bpf/** rwklx,

  # Deny cgroup
  deny /sys/fs/cgroup/** wklx,

  # ---- Mount / namespace / kernel attack surface ----
  # Deny mount/unshare/setns
  deny mount,
  deny umount,
  deny pivot_root,
  deny /bin/mount x,
  deny /usr/bin/mount x,
  deny /bin/umount x,
  deny /usr/bin/umount x,

  # ---- Device / raw block access ----
  # Deny /dev
  deny /dev/mem rwklx,
  deny /dev/kmem rwklx,
  deny /dev/kmsg rwklx,
  deny /dev/port rwklx,
  deny /dev/bpf* rwklx,

  # ---- Allow typical filesystem access ----
  / r,
  /** rixwkl,

  # ---- Deny writing to sensitive host-like locations (defense in depth) ----
  deny /etc/apparmor/** rwklx,
  deny /sys/kernel/security/** rwklx,

  # ---- End ----
}`

const (
	AppArmorDir     = "/etc/raind/lsm/apparmor"
	AppArmorProfile = "raind-default"
)

type AppArmorHandler interface {
	EnsureRaindDefaultProfile() error
}

func NewAppArmorManager() *AppArmorManager {
	return &AppArmorManager{
		filesystemHandler: utils.NewFilesystemExecutor(),
	}
}

type AppArmorManager struct {
	filesystemHandler utils.FilesystemHandler
}

// EnsureRaindDefaultProfile writes /etc/raind/lsm/apparmor/raind-default and load it
// with apparmor_parser if not already loaded
func (m *AppArmorManager) EnsureRaindDefaultProfile() error {
	// 1. check if apparmor is enabled
	if !m.isAAEnabled() {
		return fmt.Errorf("apparmor is not enabled on this host")
	}

	// 2. write apparmor profile to runtime dir
	if err := m.writeFileAtomic(filepath.Join(AppArmorDir, AppArmorProfile), []byte(raindDefaultProfile), 0644); err != nil {
		return fmt.Errorf("write profile: %w", err)
	}

	// 3. check if the profile is already loaded
	loaded, err := m.isProfilleLoaded(AppArmorProfile)
	if err != nil {
		return fmt.Errorf("check profile loaded: %w", err)
	}
	if loaded {
		return nil
	}

	// 4. load
	if err := m.loadProfileWithParser(filepath.Join(AppArmorDir, AppArmorProfile)); err != nil {
		return err
	}

	// 5. check if the profile loaded success
	loaded, err = m.isProfilleLoaded(AppArmorProfile)
	if err != nil {
		return fmt.Errorf("check profile loaded: %w", err)
	}
	if !loaded {
		return fmt.Errorf("apparmor_parser succeeded but profile %q not found in kernel list", AppArmorProfile)
	}

	return nil
}

func (m *AppArmorManager) isAAEnabled() bool {
	b, err := m.filesystemHandler.ReadFile("/sys/module/apparmor/parameters/enabled")
	if err == nil {
		s := strings.TrimSpace(string(b))
		if s == "Y" || s == "y" || s == "1" {
			return true
		}
	}
	return false
}

func (m *AppArmorManager) isProfilleLoaded(profileName string) (bool, error) {
	b, err := m.filesystemHandler.ReadFile("/sys/kernel/security/apparmor/profiles")
	if err != nil {
		return false, nil
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, profileName+" ") || line == profileName {
			return true, nil
		}
	}
	return false, nil
}

func (m *AppArmorManager) loadProfileWithParser(profilePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "apparmor_parser", "-r", profilePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("apparmor_parser timed out: %s", out.String())
		}
		return fmt.Errorf("apparmor_parser failed: %w: %s", err, out.String())
	}
	return nil
}

func (m *AppArmorManager) writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := m.filesystemHandler.MkdirAll(dir, 0755); err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := m.filesystemHandler.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := m.filesystemHandler.Rename(tmp, path); err != nil {
		_ = m.filesystemHandler.Remove(tmp)
		return err
	}
	return nil
}
