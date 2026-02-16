package container

import (
	"droplet/internal/spec"
	"fmt"
	"runtime"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Linux seccomp constants
const (
	SECCOMP_SET_MODE_FILTER   = 1
	SECCOMP_FILTER_FLAG_TSYNC = 1 << 0
)

// BPF structs (linux/filter.h)
type sockFilter struct {
	Code uint16
	Jt   uint8
	Jf   uint8
	K    uint32
}

type sockFprog struct {
	Len    uint16
	Filter *sockFilter
}

// BPF instruction helpers
const (
	// instruction classes
	bpfLD  = 0x00
	bpfRET = 0x06
	bpfJMP = 0x05

	// ld/ldx fields
	bpfW   = 0x00
	bpfABS = 0x20

	// jmp fields
	bpfJEQ = 0x10

	// ret codes
	bpfK = 0x00

	// seccomp return actions (linux/seccomp.h)
	SECCOMP_RET_KILL_PROCESS = 0x80000000
	SECCOMP_RET_KILL_THREAD  = 0x00000000
	SECCOMP_RET_TRAP         = 0x00030000
	SECCOMP_RET_ERRNO        = 0x00050000
	SECCOMP_RET_ALLOW        = 0x7fff0000
)

// seccomp_data offsets (linux/seccomp.h)
const (
	seccompDataNrOffset   = 0
	seccompDataArchOffset = 4
)

// audit arch constants (linux/audit.h)
const (
	AUDIT_ARCH_X86_64  = 0xc000003e
	AUDIT_ARCH_AARCH64 = 0xc00000b7
	AUDIT_ARCH_RISCV64 = 0xc00000f3
)

var syscallNameToNr = map[string]uint32{
	"bpf":               uint32(unix.SYS_BPF),
	"perf_event_open":   uint32(unix.SYS_PERF_EVENT_OPEN),
	"kexec_load":        uint32(unix.SYS_KEXEC_LOAD),
	"open_by_handle_at": uint32(unix.SYS_OPEN_BY_HANDLE_AT),
	"ptrace":            uint32(unix.SYS_PTRACE),
	"process_vm_readv":  uint32(unix.SYS_PROCESS_VM_READV),
	"process_vm_writev": uint32(unix.SYS_PROCESS_VM_WRITEV),
	"userfaultfd":       uint32(unix.SYS_USERFAULTFD),
	"init_module":       uint32(unix.SYS_INIT_MODULE),
	"finit_module":      uint32(unix.SYS_FINIT_MODULE),
	"delete_module":     uint32(unix.SYS_DELETE_MODULE),
	"name_to_handle_at": uint32(unix.SYS_NAME_TO_HANDLE_AT),
	"kcmp":              uint32(unix.SYS_KCMP),
	"reboot":            uint32(unix.SYS_REBOOT),
	"swapon":            uint32(unix.SYS_SWAPON),
	"swapoff":           uint32(unix.SYS_SWAPOFF),
	"mount":             uint32(unix.SYS_MOUNT),
	"unshare":           uint32(unix.SYS_UNSHARE),
	"setns":             uint32(unix.SYS_SETNS),
}

type SeccompHandler interface {
	InstallDenyFilter(seccompConfig spec.SeccompObject) error
}

func NewSeccompManager() *SeccompManager {
	return &SeccompManager{}
}

type SeccompManager struct{}

func (m *SeccompManager) InstallDenyFilter(seccompConfig spec.SeccompObject) error {
	arch, err := m.auditArchForGOARCH(runtime.GOARCH)
	if err != nil {
		return err
	}

	// 1. no_new_privs is required for unprivileged seccomp filter install
	if err := unix.Prctl(unix.PR_SET_NO_NEW_PRIVS, 1, 0, 0, 0); err != nil {
		return fmt.Errorf("prctl(PR_SET_NO_NEW_PRIVS) failed: %w", err)
	}

	// 2. build classic BPF program
	//    - verify arch; if mismatch, kill (safe default)
	//    - load syscall number; if it matches any blocked syscall, return ERRNO(EPERM)
	//    - other; ALLOW
	blocked := []uint32{}
	seen := map[uint32]struct{}{}

	for _, rule := range seccompConfig.Syscalls {
		if rule.Action != "SCMP_ACT_ERRNO" {
			continue
		}

		for _, name := range rule.Names {
			nr, ok := m.resolveSyscallNr(name)
			if !ok {
				return fmt.Errorf("unknown/unsupported syscall name %s: %q", runtime.GOARCH, name)
			}
			if _, exists := seen[nr]; exists {
				continue
			}
			seen[nr] = struct{}{}
			blocked = append(blocked, nr)
		}
	}
	if len(blocked) == 0 {
		return nil
	}

	prog := make([]sockFilter, 0, 8+len(blocked)*2)

	// A = arch
	prog = append(prog, m.bpfStmt(bpfLD|bpfW|bpfABS, seccompDataArchOffset))
	// if (A == arch) goto +1 else kill
	prog = append(prog, m.bpfJump(bpfJMP|bpfJEQ|bpfK, arch, 1, 0))
	prog = append(prog, m.bpfStmt(bpfRET|bpfK, SECCOMP_RET_KILL_PROCESS))

	// A = nr
	prog = append(prog, m.bpfStmt(bpfLD|bpfW|bpfABS, seccompDataNrOffset))

	// For each blocked syscall:
	// if (A == SYS_xxx) return ERRNO(EPERM)
	denyAction := SECCOMP_RET_ERRNO | uint32(unix.EPERM)
	for _, nr := range blocked {
		prog = append(prog, m.bpfJump(bpfJMP|bpfJEQ|bpfK, nr, 0, 1))
		prog = append(prog, m.bpfStmt(bpfRET|bpfK, denyAction))
	}

	// allow
	prog = append(prog, m.bpfStmt(bpfRET|bpfK, SECCOMP_RET_ALLOW))

	fp := sockFprog{
		Len:    uint16(len(prog)),
		Filter: &prog[0],
	}

	// 3. Call seccomp(SECCOMP_SET_MODE_FILTER, 0, &fp)
	//    Use raw syscall because x/sys/unix does not guarantee wrapper availability.
	_, _, errno := unix.Syscall(unix.SYS_SECCOMP,
		uintptr(SECCOMP_SET_MODE_FILTER),
		uintptr(0),
		uintptr(unsafe.Pointer(&fp)),
	)
	if errno != 0 {
		return fmt.Errorf("seccomp(SECCOMP_SET_MODE_FILTER) failed: %v", errno)
	}

	return nil
}

func (m *SeccompManager) auditArchForGOARCH(goarch string) (uint32, error) {
	switch goarch {
	case "amd64":
		return AUDIT_ARCH_X86_64, nil
	case "arm64":
		return AUDIT_ARCH_AARCH64, nil
	case "riscv64":
		return AUDIT_ARCH_RISCV64, nil
	default:
		return 0, fmt.Errorf("unsupported GOARCH for seccomp audit arch: %s", goarch)
	}
}

func (m *SeccompManager) bpfStmt(code uint16, k uint32) sockFilter {
	return sockFilter{Code: code, Jt: 0, Jf: 0, K: k}
}

func (m *SeccompManager) bpfJump(code uint16, k uint32, jt uint8, jf uint8) sockFilter {
	return sockFilter{Code: code, Jt: jt, Jf: jf, K: k}
}

func (m *SeccompManager) resolveSyscallNr(name string) (uint32, bool) {
	normalizeSyscallName := func(s string) string {
		s = strings.TrimSpace(s)
		s = strings.ToLower(s)
		return s
	}

	n := normalizeSyscallName(name)
	nr, ok := syscallNameToNr[n]
	return nr, ok
}
