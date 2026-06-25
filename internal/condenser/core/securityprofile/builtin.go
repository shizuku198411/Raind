package securityprofile

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strings"
)

const (
	DefaultStoreDir = "/etc/raind/security-profiles"
	StoreDirEnv     = "RAIND_SECURITY_PROFILE_DIR"
)

type Service struct {
	storeDir string
}

func NewService() *Service {
	return NewServiceWithStoreDir(resolveStoreDir())
}

func NewServiceWithStoreDir(storeDir string) *Service {
	return &Service{storeDir: storeDir}
}

func resolveStoreDir() string {
	if v := os.Getenv(StoreDirEnv); v != "" {
		return v
	}
	return DefaultStoreDir
}

func (s *Service) List() []ProfileSummary {
	profiles := builtinProfiles()
	customProfiles, err := s.listCustomProfiles()
	if err == nil {
		profiles = append(profiles, customProfiles...)
	}

	out := make([]ProfileSummary, 0, len(profiles))
	for _, profile := range profiles {
		out = append(out, profile.Summary())
	}
	return out
}

func (s *Service) Get(name string) (SecurityProfile, error) {
	if profile, ok := builtinProfile(name); ok {
		return profile, nil
	}
	return s.getCustomProfile(name, map[string]struct{}{})
}

func (s *Service) Resolve(name string) (SecurityProfile, error) {
	return s.Get(name)
}

func (s *Service) Register(manifest CustomProfileManifest) (SecurityProfile, error) {
	if err := s.validateManifest(manifest); err != nil {
		return SecurityProfile{}, err
	}
	name := manifest.ProfileName()
	if err := os.MkdirAll(s.storeDir, 0755); err != nil {
		return SecurityProfile{}, fmt.Errorf("create security profile store: %w", err)
	}
	path := s.profilePath(name)
	if _, err := os.Stat(path); err == nil {
		return SecurityProfile{}, fmt.Errorf("security profile already exists: %s", name)
	} else if err != nil && !os.IsNotExist(err) {
		return SecurityProfile{}, fmt.Errorf("stat security profile: %w", err)
	}

	if err := writeManifestFile(path, manifest); err != nil {
		return SecurityProfile{}, err
	}
	profile, err := s.Get(name)
	if err != nil {
		_ = os.Remove(path)
		return SecurityProfile{}, err
	}
	return profile, nil
}

func (s *Service) Delete(name string) error {
	if name == "" {
		return fmt.Errorf("missing security profile name")
	}
	if _, ok := builtinProfile(name); ok {
		return fmt.Errorf("cannot delete built-in security profile: %s", name)
	}
	if !validProfileName(name) {
		return fmt.Errorf("invalid security profile name: %s", name)
	}
	path := s.profilePath(name)
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("unknown security profile: %s", name)
		}
		return fmt.Errorf("delete security profile: %w", err)
	}
	return nil
}

func (s *Service) validateManifest(manifest CustomProfileManifest) error {
	name := manifest.ProfileName()
	if name == "" {
		return fmt.Errorf("missing security profile name")
	}
	if !validProfileName(name) {
		return fmt.Errorf("invalid security profile name: %s", name)
	}
	if _, ok := builtinProfile(name); ok {
		return fmt.Errorf("cannot overwrite built-in security profile: %s", name)
	}
	extends := manifest.ProfileExtends()
	if extends == "" {
		return fmt.Errorf("security profile extends is required")
	}
	if extends == name {
		return fmt.Errorf("security profile cannot extend itself: %s", name)
	}
	if _, err := s.Get(extends); err != nil {
		return fmt.Errorf("unknown parent security profile %q: %w", extends, err)
	}
	for _, cap := range append(slices.Clone(manifest.ProfileAddCap()), manifest.ProfileDropCap()...) {
		if !validCapabilityName(cap) {
			return fmt.Errorf("invalid capability name: %s", cap)
		}
	}
	return nil
}

func (s *Service) listCustomProfiles() ([]SecurityProfile, error) {
	entries, err := os.ReadDir(s.storeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read security profile store: %w", err)
	}
	profiles := make([]SecurityProfile, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		manifest, err := readManifestFile(filepath.Join(s.storeDir, entry.Name()))
		if err != nil {
			continue
		}
		profile, err := s.resolveCustomManifest(manifest, map[string]struct{}{})
		if err != nil {
			continue
		}
		profiles = append(profiles, profile)
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
	return profiles, nil
}

func (s *Service) getCustomProfile(name string, seen map[string]struct{}) (SecurityProfile, error) {
	if name == "" {
		return DefaultSecurityProfile(), nil
	}
	if !validProfileName(name) {
		return SecurityProfile{}, fmt.Errorf("unknown security profile: %s", name)
	}
	manifest, err := readManifestFile(s.profilePath(name))
	if err != nil {
		if os.IsNotExist(err) {
			return SecurityProfile{}, fmt.Errorf("unknown security profile: %s", name)
		}
		return SecurityProfile{}, err
	}
	return s.resolveCustomManifest(manifest, seen)
}

func (s *Service) resolveCustomManifest(manifest CustomProfileManifest, seen map[string]struct{}) (SecurityProfile, error) {
	if err := s.validateCustomManifestShape(manifest); err != nil {
		return SecurityProfile{}, err
	}
	name := manifest.ProfileName()
	if _, ok := seen[name]; ok {
		return SecurityProfile{}, fmt.Errorf("security profile extends cycle detected: %s", name)
	}
	seen[name] = struct{}{}
	defer delete(seen, name)

	extends := manifest.ProfileExtends()
	parent, ok := builtinProfile(extends)
	var err error
	if !ok {
		parent, err = s.getCustomProfile(extends, seen)
		if err != nil {
			return SecurityProfile{}, err
		}
	}
	profile := parent
	profile.Name = name
	profile.Type = ProfileTypeCustom
	profile.Extends = extends
	profile.AddCap = slices.Clone(manifest.ProfileAddCap())
	profile.DropCap = slices.Clone(manifest.ProfileDropCap())
	profile.Capabilities.Base = mergeCapabilityDelta(profile.Capabilities.Base, profile.AddCap, profile.DropCap)
	profile.Seccomp = CloneSeccompObject(profile.Seccomp)
	return profile, nil
}

func (s *Service) validateCustomManifestShape(manifest CustomProfileManifest) error {
	name := manifest.ProfileName()
	if name == "" || !validProfileName(name) {
		return fmt.Errorf("invalid security profile name: %s", name)
	}
	if _, ok := builtinProfile(name); ok {
		return fmt.Errorf("cannot overwrite built-in security profile: %s", name)
	}
	if manifest.ProfileExtends() == "" {
		return fmt.Errorf("security profile extends is required")
	}
	return nil
}

func (s *Service) profilePath(name string) string {
	return filepath.Join(s.storeDir, name+".yaml")
}

func builtinProfiles() []SecurityProfile {
	return []SecurityProfile{
		DefaultSecurityProfile(),
		DevSecurityProfile(),
		DeploySecurityProfile(),
		RestrictedSecurityProfile(),
		PrivilegedSecurityProfile(),
		UnconfinedSecurityProfile(),
	}
}

func builtinProfile(name string) (SecurityProfile, bool) {
	switch name {
	case "", ProfileDefault:
		return DefaultSecurityProfile(), true
	case ProfileDev:
		return DevSecurityProfile(), true
	case ProfileDeploy:
		return DeploySecurityProfile(), true
	case ProfileRestricted:
		return RestrictedSecurityProfile(), true
	case ProfilePrivileged:
		return PrivilegedSecurityProfile(), true
	case ProfileUnconfined:
		return UnconfinedSecurityProfile(), true
	default:
		return SecurityProfile{}, false
	}
}

func (p SecurityProfile) Summary() ProfileSummary {
	return ProfileSummary{
		Name:              p.Name,
		Type:              p.Type,
		CapabilitiesCount: len(p.Capabilities.Base),
		SeccompEnabled:    p.Seccomp != nil,
		AppArmorProfile:   p.AppArmorProfile,
		NoNewPrivileges:   p.NoNewPrivileges,
	}
}

func DevSecurityProfile() SecurityProfile {
	profile := DefaultSecurityProfile()
	profile.Name = ProfileDev
	profile.NoNewPrivileges = true
	return profile
}

func DeploySecurityProfile() SecurityProfile {
	profile := DefaultSecurityProfile()
	profile.Name = ProfileDeploy
	profile.Capabilities.Base = dropCapabilities(profile.Capabilities.Base, "CAP_NET_RAW", "CAP_MKNOD")
	profile.NoNewPrivileges = true
	return profile
}

func RestrictedSecurityProfile() SecurityProfile {
	profile := DefaultSecurityProfile()
	profile.Name = ProfileRestricted
	profile.Capabilities.Base = []string{}
	profile.NoNewPrivileges = true
	return profile
}

func PrivilegedSecurityProfile() SecurityProfile {
	profile := DefaultSecurityProfile()
	profile.Name = ProfilePrivileged
	profile.Capabilities.Base = allCapabilities()
	profile.Seccomp = nil
	profile.AppArmorProfile = ""
	profile.NoNewPrivileges = false
	return profile
}

func UnconfinedSecurityProfile() SecurityProfile {
	profile := DefaultSecurityProfile()
	profile.Name = ProfileUnconfined
	profile.Seccomp = nil
	profile.AppArmorProfile = ""
	profile.NoNewPrivileges = false
	return profile
}

func allCapabilities() []string {
	return []string{
		"CAP_CHOWN",
		"CAP_DAC_OVERRIDE",
		"CAP_DAC_READ_SEARCH",
		"CAP_FOWNER",
		"CAP_FSETID",
		"CAP_KILL",
		"CAP_SETGID",
		"CAP_SETUID",
		"CAP_SETPCAP",
		"CAP_LINUX_IMMUTABLE",
		"CAP_NET_BIND_SERVICE",
		"CAP_NET_BROADCAST",
		"CAP_NET_ADMIN",
		"CAP_NET_RAW",
		"CAP_IPC_LOCK",
		"CAP_IPC_OWNER",
		"CAP_SYS_MODULE",
		"CAP_SYS_RAWIO",
		"CAP_SYS_CHROOT",
		"CAP_SYS_PTRACE",
		"CAP_SYS_PACCT",
		"CAP_SYS_ADMIN",
		"CAP_SYS_BOOT",
		"CAP_SYS_NICE",
		"CAP_SYS_RESOURCE",
		"CAP_SYS_TIME",
		"CAP_SYS_TTY_CONFIG",
		"CAP_MKNOD",
		"CAP_LEASE",
		"CAP_AUDIT_WRITE",
		"CAP_AUDIT_CONTROL",
		"CAP_SETFCAP",
		"CAP_MAC_OVERRIDE",
		"CAP_MAC_ADMIN",
		"CAP_SYSLOG",
		"CAP_WAKE_ALARM",
		"CAP_BLOCK_SUSPEND",
		"CAP_AUDIT_READ",
		"CAP_PERFMON",
		"CAP_BPF",
		"CAP_CHECKPOINT_RESTORE",
	}
}

func dropCapabilities(base []string, drops ...string) []string {
	dropSet := make(map[string]struct{}, len(drops))
	for _, cap := range drops {
		dropSet[cap] = struct{}{}
	}
	out := make([]string, 0, len(base))
	for _, cap := range base {
		if _, ok := dropSet[cap]; ok {
			continue
		}
		out = append(out, cap)
	}
	return out
}

func mergeCapabilityDelta(base []string, addCaps []string, dropCaps []string) []string {
	out := slices.Clone(base)
	dropSet := make(map[string]struct{}, len(dropCaps))
	for _, cap := range dropCaps {
		dropSet[cap] = struct{}{}
	}
	out = dropCapabilities(out, dropCaps...)
	seen := make(map[string]struct{}, len(out)+len(addCaps))
	for _, cap := range out {
		seen[cap] = struct{}{}
	}
	for _, cap := range addCaps {
		if _, dropped := dropSet[cap]; dropped {
			continue
		}
		if _, ok := seen[cap]; ok {
			continue
		}
		out = append(out, cap)
		seen[cap] = struct{}{}
	}
	return out
}

var profileNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,62}$`)

func validProfileName(name string) bool {
	return profileNamePattern.MatchString(name)
}

func validCapabilityName(cap string) bool {
	if cap == "" || !strings.HasPrefix(cap, "CAP_") {
		return false
	}
	for _, r := range cap[4:] {
		if r < 'A' || r > 'Z' {
			if r < '0' || r > '9' {
				if r != '_' {
					return false
				}
			}
		}
	}
	return true
}

func DefaultSecurityProfile() SecurityProfile {
	ep := uint32(1)
	return SecurityProfile{
		Name: ProfileDefault,
		Type: ProfileTypeBuiltIn,
		Capabilities: CapabilityProfile{
			Base: []string{
				"CAP_CHOWN",
				"CAP_DAC_OVERRIDE",
				"CAP_FSETID",
				"CAP_FOWNER",
				"CAP_MKNOD",
				"CAP_NET_RAW",
				"CAP_SETGID",
				"CAP_SETUID",
				"CAP_SETFCAP",
				"CAP_SETPCAP",
				"CAP_NET_BIND_SERVICE",
				"CAP_SYS_CHROOT",
				"CAP_KILL",
				"CAP_AUDIT_WRITE",
			},
		},
		Seccomp: &SeccompObject{
			DefaultAction:   "SCMP_ACT_ALLOW",
			DefaultErrnoRet: &ep,
			Architectures:   defaultSeccompArchitectures(),
			Syscalls: []SeccompSyscallObject{
				{
					Names: []string{
						"bpf",
						"perf_event_open",
						"kexec_load",
						"open_by_handle_at",
						"ptrace",
						"process_vm_readv",
						"process_vm_writev",
						"userfaultfd",
						"reboot",
						"swapon",
						"swapoff",
						"open_by_handle_at",
						"name_to_handle_at",
						"init_module",
						"finit_module",
						"delete_module",
						"kcmp",
						"mount",
						"unshare",
						"setns",
					},
					Action:   "SCMP_ACT_ERRNO",
					ErrnoRet: &ep,
				},
			},
		},
		AppArmorProfile: "raind-default",
	}
}

func defaultSeccompArchitectures() []string {
	switch runtime.GOARCH {
	case "amd64":
		return []string{"SCMP_ARCH_X86_64"}
	case "arm64":
		return []string{"SCMP_ARCH_AARCH64"}
	case "riscv64":
		return []string{"SCMP_ARCH_RISCV64"}
	default:
		return []string{""}
	}
}

func CloneSeccompObject(in *SeccompObject) *SeccompObject {
	if in == nil {
		return nil
	}
	out := *in
	if in.DefaultErrnoRet != nil {
		v := *in.DefaultErrnoRet
		out.DefaultErrnoRet = &v
	}
	out.Architectures = slices.Clone(in.Architectures)
	out.Flags = slices.Clone(in.Flags)
	out.Syscalls = make([]SeccompSyscallObject, len(in.Syscalls))
	for i, syscall := range in.Syscalls {
		out.Syscalls[i] = cloneSeccompSyscallObject(syscall)
	}
	return &out
}

func cloneSeccompSyscallObject(in SeccompSyscallObject) SeccompSyscallObject {
	out := in
	if in.ErrnoRet != nil {
		v := *in.ErrnoRet
		out.ErrnoRet = &v
	}
	out.Names = slices.Clone(in.Names)
	out.Args = make([]SeccompArgObject, len(in.Args))
	for i, arg := range in.Args {
		out.Args[i] = arg
		if arg.ValueTwo != nil {
			v := *arg.ValueTwo
			out.Args[i].ValueTwo = &v
		}
	}
	if in.Include != nil {
		include := *in.Include
		include.Architectures = slices.Clone(in.Include.Architectures)
		include.Caps = slices.Clone(in.Include.Caps)
		out.Include = &include
	}
	if in.Excludes != nil {
		excludes := *in.Excludes
		excludes.Architectures = slices.Clone(in.Excludes.Architectures)
		excludes.Caps = slices.Clone(in.Excludes.Caps)
		out.Excludes = &excludes
	}
	return out
}
