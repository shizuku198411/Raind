package container

import (
	"fmt"
	"github.com/syndtr/gocapability/capability"
)

var capNameMap = map[string]capability.Cap{
	"CAP_CHOWN":            capability.CAP_CHOWN,
	"CAP_DAC_OVERRIDE":     capability.CAP_DAC_OVERRIDE,
	"CAP_FSETID":           capability.CAP_FSETID,
	"CAP_FOWNER":           capability.CAP_FOWNER,
	"CAP_MKNOD":            capability.CAP_MKNOD,
	"CAP_NET_RAW":          capability.CAP_NET_RAW,
	"CAP_SETGID":           capability.CAP_SETGID,
	"CAP_SETUID":           capability.CAP_SETUID,
	"CAP_SETFCAP":          capability.CAP_SETFCAP,
	"CAP_SETPCAP":          capability.CAP_SETPCAP,
	"CAP_NET_BIND_SERVICE": capability.CAP_NET_BIND_SERVICE,
	"CAP_SYS_CHROOT":       capability.CAP_SYS_CHROOT,
	"CAP_KILL":             capability.CAP_KILL,
	"CAP_AUDIT_WRITE":      capability.CAP_AUDIT_WRITE,
}

func toCaps(names []string) []capability.Cap {
	res := make([]capability.Cap, 0, len(names))
	for _, n := range names {
		if v, ok := capNameMap[n]; ok {
			res = append(res, v)
		} else {
			fmt.Errorf("unknown capability: %s\n", n)
		}
	}
	return res
}
