package container

import (
	"raind/internal/droplet/container/security"

	"github.com/syndtr/gocapability/capability"
)

var capNameMap = security.CapNameMap

func toCaps(names []string) []capability.Cap {
	return security.ToCaps(names)
}
