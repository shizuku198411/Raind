package main

import "testing"

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
