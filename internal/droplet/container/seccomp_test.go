package container

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestSeccompManagerAuditArchForSupportedGOARCH(t *testing.T) {
	// == setup ==
	manager := &SeccompManager{}

	tests := []struct {
		goarch string
		want   uint32
	}{
		{goarch: "amd64", want: AUDIT_ARCH_X86_64},
		{goarch: "arm64", want: AUDIT_ARCH_AARCH64},
		{goarch: "riscv64", want: AUDIT_ARCH_RISCV64},
	}

	for _, tt := range tests {
		t.Run(tt.goarch, func(t *testing.T) {
			// == exercise ==
			got, err := manager.auditArchForGOARCH(tt.goarch)

			// == assert ==
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSeccompManagerResolveSyscallNrNormalizesNames(t *testing.T) {
	// == setup ==
	manager := &SeccompManager{}

	// == exercise ==
	nr, ok := manager.resolveSyscallNr("  MOUNT ")

	// == assert ==
	require.True(t, ok)
	assert.Equal(t, uint32(unix.SYS_MOUNT), nr)
}

func TestSeccompManagerResolveSyscallNrReturnsFalseForUnsupportedName(t *testing.T) {
	// == setup ==
	manager := &SeccompManager{}

	// == exercise ==
	_, ok := manager.resolveSyscallNr("unsupported_syscall")

	// == assert ==
	assert.False(t, ok)
}

func TestSeccompManagerBpfHelpersBuildExpectedInstructions(t *testing.T) {
	// == setup ==
	manager := &SeccompManager{}

	// == exercise ==
	stmt := manager.bpfStmt(bpfLD|bpfW|bpfABS, seccompDataNrOffset)
	jump := manager.bpfJump(bpfJMP|bpfJEQ|bpfK, 42, 1, 2)

	// == assert ==
	assert.Equal(t, sockFilter{Code: bpfLD | bpfW | bpfABS, K: seccompDataNrOffset}, stmt)
	assert.Equal(t, sockFilter{Code: bpfJMP | bpfJEQ | bpfK, K: 42, Jt: 1, Jf: 2}, jump)
}

func TestSeccompManagerSupportsCurrentGOARCH(t *testing.T) {
	// == setup ==
	manager := &SeccompManager{}

	// == exercise ==
	_, err := manager.auditArchForGOARCH(runtime.GOARCH)

	// == assert ==
	require.NoError(t, err)
}
