package security

import (
	"fmt"
	"github.com/syndtr/gocapability/capability"
)

var CapNameMap = map[string]capability.Cap{
	"CAP_CHOWN":              capability.CAP_CHOWN,
	"CAP_DAC_OVERRIDE":       capability.CAP_DAC_OVERRIDE,
	"CAP_DAC_READ_SEARCH":    capability.CAP_DAC_READ_SEARCH,
	"CAP_FOWNER":             capability.CAP_FOWNER,
	"CAP_FSETID":             capability.CAP_FSETID,
	"CAP_KILL":               capability.CAP_KILL,
	"CAP_SETGID":             capability.CAP_SETGID,
	"CAP_SETUID":             capability.CAP_SETUID,
	"CAP_SETPCAP":            capability.CAP_SETPCAP,
	"CAP_LINUX_IMMUTABLE":    capability.CAP_LINUX_IMMUTABLE,
	"CAP_NET_BIND_SERVICE":   capability.CAP_NET_BIND_SERVICE,
	"CAP_NET_BROADCAST":      capability.CAP_NET_BROADCAST,
	"CAP_NET_ADMIN":          capability.CAP_NET_ADMIN,
	"CAP_NET_RAW":            capability.CAP_NET_RAW,
	"CAP_IPC_LOCK":           capability.CAP_IPC_LOCK,
	"CAP_IPC_OWNER":          capability.CAP_IPC_OWNER,
	"CAP_SYS_MODULE":         capability.CAP_SYS_MODULE,
	"CAP_SYS_RAWIO":          capability.CAP_SYS_RAWIO,
	"CAP_SYS_CHROOT":         capability.CAP_SYS_CHROOT,
	"CAP_SYS_PTRACE":         capability.CAP_SYS_PTRACE,
	"CAP_SYS_PACCT":          capability.CAP_SYS_PACCT,
	"CAP_SYS_ADMIN":          capability.CAP_SYS_ADMIN,
	"CAP_SYS_BOOT":           capability.CAP_SYS_BOOT,
	"CAP_SYS_NICE":           capability.CAP_SYS_NICE,
	"CAP_SYS_RESOURCE":       capability.CAP_SYS_RESOURCE,
	"CAP_SYS_TIME":           capability.CAP_SYS_TIME,
	"CAP_SYS_TTY_CONFIG":     capability.CAP_SYS_TTY_CONFIG,
	"CAP_MKNOD":              capability.CAP_MKNOD,
	"CAP_LEASE":              capability.CAP_LEASE,
	"CAP_AUDIT_WRITE":        capability.CAP_AUDIT_WRITE,
	"CAP_AUDIT_CONTROL":      capability.CAP_AUDIT_CONTROL,
	"CAP_SETFCAP":            capability.CAP_SETFCAP,
	"CAP_MAC_OVERRIDE":       capability.CAP_MAC_OVERRIDE,
	"CAP_MAC_ADMIN":          capability.CAP_MAC_ADMIN,
	"CAP_SYSLOG":             capability.CAP_SYSLOG,
	"CAP_WAKE_ALARM":         capability.CAP_WAKE_ALARM,
	"CAP_BLOCK_SUSPEND":      capability.CAP_BLOCK_SUSPEND,
	"CAP_AUDIT_READ":         capability.CAP_AUDIT_READ,
	"CAP_PERFMON":            capability.CAP_PERFMON,
	"CAP_BPF":                capability.CAP_BPF,
	"CAP_CHECKPOINT_RESTORE": capability.CAP_CHECKPOINT_RESTORE,
}

func ToCaps(names []string) []capability.Cap {
	res := make([]capability.Cap, 0, len(names))
	for _, n := range names {
		if v, ok := CapNameMap[n]; ok {
			res = append(res, v)
		} else {
			fmt.Errorf("unknown capability: %s\n", n)
		}
	}
	return res
}
