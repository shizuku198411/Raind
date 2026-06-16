package securityprofile

import (
	"fmt"
	"runtime"
	"slices"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) List() []ProfileSummary {
	profiles := []SecurityProfile{
		DefaultSecurityProfile(),
		DevSecurityProfile(),
		DeploySecurityProfile(),
	}
	out := make([]ProfileSummary, 0, len(profiles))
	for _, profile := range profiles {
		out = append(out, profile.Summary())
	}
	return out
}

func (s *Service) Get(name string) (SecurityProfile, error) {
	switch name {
	case "", ProfileDefault:
		return DefaultSecurityProfile(), nil
	case ProfileDev:
		return DevSecurityProfile(), nil
	case ProfileDeploy:
		return DeploySecurityProfile(), nil
	default:
		return SecurityProfile{}, fmt.Errorf("unknown security profile: %s", name)
	}
}

func (s *Service) Resolve(name string) (SecurityProfile, error) {
	return s.Get(name)
}

func (p SecurityProfile) Summary() ProfileSummary {
	return ProfileSummary{
		Name:              p.Name,
		Type:              p.Type,
		CapabilitiesCount: len(p.Capabilities.Base),
		SeccompEnabled:    p.Seccomp != nil,
		AppArmorProfile:   p.AppArmorProfile,
	}
}

func DevSecurityProfile() SecurityProfile {
	profile := DefaultSecurityProfile()
	profile.Name = ProfileDev
	return profile
}

func DeploySecurityProfile() SecurityProfile {
	profile := DefaultSecurityProfile()
	profile.Name = ProfileDeploy
	profile.Capabilities.Base = dropCapabilities(profile.Capabilities.Base, "CAP_NET_RAW", "CAP_MKNOD")
	return profile
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
