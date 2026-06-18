package main

import (
	"errors"
	"os"
	"testing"
)

func TestIsInitSubcommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{name: "init", args: []string{"init", "container-id", "/tmp/fifo"}, want: true},
		{name: "create", args: []string{"create", "container-id"}, want: false},
		{name: "empty", args: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInitSubcommand(tt.args); got != tt.want {
				t.Fatalf("isInitSubcommand(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}

func TestIsNonInitialUserNamespace(t *testing.T) {
	tests := []struct {
		name   string
		uidMap string
		want   bool
	}{
		{name: "initial namespace", uidMap: "         0          0 4294967295\n", want: false},
		{name: "shifted rootless namespace", uidMap: "         0     100000      65536\n", want: true},
		{name: "login rootless namespace", uidMap: "         0       1000          1\n         1     100000      65535\n", want: true},
		{name: "empty", uidMap: "", want: false},
		{name: "malformed host id", uidMap: "0 nope 1\n", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNonInitialUserNamespace(tt.uidMap); got != tt.want {
				t.Fatalf("isNonInitialUserNamespace(%q) = %v, want %v", tt.uidMap, got, tt.want)
			}
		})
	}
}

func TestUserNamespaceDiffersFromInit(t *testing.T) {
	tests := []struct {
		name string
		self string
		init string
		want bool
	}{
		{
			name: "same initial namespace",
			self: "         0          0 4294967295\n",
			init: "         0          0 4294967295\n",
			want: false,
		},
		{
			name: "same shifted workshop namespace",
			self: "         0     100000      65536\n",
			init: "         0     100000      65536\n",
			want: false,
		},
		{
			name: "nested rootless namespace",
			self: "         0     200000      65536\n",
			init: "         0     100000      65536\n",
			want: true,
		},
		{
			name: "empty self",
			self: "",
			init: "         0     100000      65536\n",
			want: false,
		},
		{
			name: "empty init",
			self: "         0     200000      65536\n",
			init: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := userNamespaceDiffersFromInit(tt.self, tt.init); got != tt.want {
				t.Fatalf("userNamespaceDiffersFromInit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShouldSkipAuditLoggerInit(t *testing.T) {
	tests := []struct {
		name           string
		args           []string
		err            error
		inNestedUserNS bool
		want           bool
	}{
		{
			name: "init skips any logger error",
			args: []string{"init", "container-id", "/tmp/fifo"},
			err:  errors.New("open failed"),
			want: true,
		},
		{
			name:           "nested rootless namespace create skips permission error",
			args:           []string{"create", "container-id"},
			err:            os.ErrPermission,
			inNestedUserNS: true,
			want:           true,
		},
		{
			name:           "nested rootless namespace create keeps non permission error",
			args:           []string{"create", "container-id"},
			err:            errors.New("disk full"),
			inNestedUserNS: true,
			want:           false,
		},
		{
			name:           "base namespace create keeps permission error",
			args:           []string{"create", "container-id"},
			err:            os.ErrPermission,
			inNestedUserNS: false,
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldSkipAuditLoggerInitWithNamespace(tt.args, tt.err, func() bool {
				return tt.inNestedUserNS
			})
			if got != tt.want {
				t.Fatalf("shouldSkipAuditLoggerInitWithNamespace(%v, %v) = %v, want %v", tt.args, tt.err, got, tt.want)
			}
		})
	}
}
