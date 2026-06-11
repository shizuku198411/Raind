package container

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/syndtr/gocapability/capability"
)

func TestToCapsMapsKnownCapabilityNamesInOrder(t *testing.T) {
	// == exercise ==
	caps := toCaps([]string{
		"CAP_CHOWN",
		"CAP_NET_ADMIN",
		"CAP_SYS_ADMIN",
	})

	// == assert ==
	assert.Equal(t, []capability.Cap{
		capability.CAP_CHOWN,
		capability.CAP_NET_ADMIN,
		capability.CAP_SYS_ADMIN,
	}, caps)
}
