package spec

import (
	"fmt"
	"runtime"
	"slices"
)

const (
	SecurityProfileDefault = "default"
	SecurityProfileDev     = "dev"
)

type SecurityOption struct {
	ProfileName string
}

type SecurityProfile struct {
	Name            string
	Capabilities    CapabilityProfile
	Seccomp         *SeccompObject
	AppArmorProfile string
}

type CapabilityProfile struct {
	Base []string
}

func ResolveSecurityProfile(name string) (SecurityProfile, error) {
	switch name {
	case "", SecurityProfileDefault:
		return DefaultSecurityProfile(), nil
	case SecurityProfileDev:
		return DevSecurityProfile(), nil
	default:
		return SecurityProfile{}, fmt.Errorf("unknown security profile: %s", name)
	}
}

func DevSecurityProfile() SecurityProfile {
	profile := DefaultSecurityProfile()
	profile.Name = SecurityProfileDev
	return profile
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
