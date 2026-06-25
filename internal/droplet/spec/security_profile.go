package spec

import (
	"fmt"
	"runtime"
	"slices"
)

const (
	SecurityProfileDefault    = "default"
	SecurityProfileDev        = "dev"
	SecurityProfileDeploy     = "deploy"
	SecurityProfileRestricted = "restricted"
	SecurityProfilePrivileged = "privileged"
	SecurityProfileUnconfined = "unconfined"
)

type SecurityOption struct {
	ProfileName      string
	BaseCapabilities []string
	Seccomp          *SeccompObject
	AppArmorProfile  string
	NoNewPrivileges  bool
}

type SecurityProfile struct {
	Name            string
	Capabilities    CapabilityProfile
	Seccomp         *SeccompObject
	AppArmorProfile string
	NoNewPrivileges bool
}

type CapabilityProfile struct {
	Base []string
}

func ResolveSecurityOption(opt SecurityOption) (SecurityProfile, error) {
	if len(opt.BaseCapabilities) != 0 || opt.Seccomp != nil || opt.AppArmorProfile != "" || opt.NoNewPrivileges {
		return SecurityProfile{
			Name: opt.ProfileName,
			Capabilities: CapabilityProfile{
				Base: opt.BaseCapabilities,
			},
			Seccomp:         cloneSeccompObject(opt.Seccomp),
			AppArmorProfile: opt.AppArmorProfile,
			NoNewPrivileges: opt.NoNewPrivileges,
		}, nil
	}
	return ResolveSecurityProfile(opt.ProfileName)
}

func ResolveSecurityProfile(name string) (SecurityProfile, error) {
	switch name {
	case "", SecurityProfileDefault:
		return DefaultSecurityProfile(), nil
	case SecurityProfileDev:
		return DevSecurityProfile(), nil
	case SecurityProfileDeploy:
		return DeploySecurityProfile(), nil
	case SecurityProfileRestricted:
		return RestrictedSecurityProfile(), nil
	case SecurityProfilePrivileged:
		return PrivilegedSecurityProfile(), nil
	case SecurityProfileUnconfined:
		return UnconfinedSecurityProfile(), nil
	default:
		return SecurityProfile{}, fmt.Errorf("unknown security profile: %s", name)
	}
}

func DevSecurityProfile() SecurityProfile {
	profile := DefaultSecurityProfile()
	profile.Name = SecurityProfileDev
	profile.NoNewPrivileges = true
	return profile
}

func DeploySecurityProfile() SecurityProfile {
	profile := DefaultSecurityProfile()
	profile.Name = SecurityProfileDeploy
	profile.Capabilities.Base = dropCapabilities(profile.Capabilities.Base, "CAP_NET_RAW", "CAP_MKNOD")
	profile.NoNewPrivileges = true
	return profile
}

func RestrictedSecurityProfile() SecurityProfile {
	profile := DefaultSecurityProfile()
	profile.Name = SecurityProfileRestricted
	profile.Capabilities.Base = []string{}
	profile.NoNewPrivileges = true
	return profile
}

func PrivilegedSecurityProfile() SecurityProfile {
	profile := DefaultSecurityProfile()
	profile.Name = SecurityProfilePrivileged
	profile.Capabilities.Base = allCapabilities()
	profile.Seccomp = nil
	profile.AppArmorProfile = ""
	profile.NoNewPrivileges = false
	return profile
}

func UnconfinedSecurityProfile() SecurityProfile {
	profile := DefaultSecurityProfile()
	profile.Name = SecurityProfileUnconfined
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

func DefaultSecurityProfile() SecurityProfile {
	ep := uint32(1)
	return SecurityProfile{
		Name: SecurityProfileDefault,
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

func cloneSeccompObject(in *SeccompObject) *SeccompObject {
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
