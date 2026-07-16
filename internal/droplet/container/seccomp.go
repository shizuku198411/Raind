package container

import (
	"raind/internal/droplet/container/security"
	"raind/internal/droplet/spec"
)

type SeccompHandler interface {
	InstallDenyFilter(seccompConfig spec.SeccompObject) error
}

type SeccompManager struct {
	manager security.SeccompManager
}

func NewSeccompManager() *SeccompManager {
	return &SeccompManager{}
}

func (m *SeccompManager) InstallDenyFilter(seccompConfig spec.SeccompObject) error {
	return m.manager.InstallDenyFilter(seccompConfig)
}

type sockFilter = security.SockFilter
type sockFprog = security.SockFprog

const (
	SECCOMP_SET_MODE_FILTER   = security.SECCOMP_SET_MODE_FILTER
	SECCOMP_FILTER_FLAG_TSYNC = security.SECCOMP_FILTER_FLAG_TSYNC
	SECCOMP_RET_KILL_PROCESS  = security.SECCOMP_RET_KILL_PROCESS
	SECCOMP_RET_KILL_THREAD   = security.SECCOMP_RET_KILL_THREAD
	SECCOMP_RET_TRAP          = security.SECCOMP_RET_TRAP
	SECCOMP_RET_ERRNO         = security.SECCOMP_RET_ERRNO
	SECCOMP_RET_ALLOW         = security.SECCOMP_RET_ALLOW

	bpfLD  = 0x00
	bpfRET = 0x06
	bpfJMP = 0x05
	bpfW   = 0x00
	bpfABS = 0x20
	bpfJEQ = 0x10
	bpfK   = 0x00

	seccompDataNrOffset   = 0
	seccompDataArchOffset = 4

	AUDIT_ARCH_X86_64  = 0xc000003e
	AUDIT_ARCH_AARCH64 = 0xc00000b7
	AUDIT_ARCH_RISCV64 = 0xc00000f3

	linuxSysClone3 = 435
)

func (m *SeccompManager) auditArchForGOARCH(goarch string) (uint32, error) {
	return m.manager.AuditArchForGOARCH(goarch)
}

func (m *SeccompManager) bpfStmt(code uint16, k uint32) sockFilter {
	return m.manager.BPFStmt(code, k)
}

func (m *SeccompManager) bpfJump(code uint16, k uint32, jt uint8, jf uint8) sockFilter {
	return m.manager.BPFJump(code, k, jt, jf)
}

func (m *SeccompManager) resolveSyscallNr(name string) (uint32, bool) {
	return m.manager.ResolveSyscallNr(name)
}

func (m *SeccompManager) errnoForRule(rule spec.SeccompSyscallObject) uint32 {
	return m.manager.ErrnoForRule(rule)
}
