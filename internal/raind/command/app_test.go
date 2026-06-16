package command

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAppContainsExpectedTopLevelCommands(t *testing.T) {
	app := NewApp()
	names := map[string]bool{}
	for _, cmd := range app.Commands {
		names[cmd.Name] = true
	}

	for _, name := range []string{"container", "image", "network", "resource", "security", "policy", "bottle", "logs", "completion"} {
		assert.True(t, names[name], "missing command %s", name)
	}
}

func TestCompletionCommandsRenderSupportedShells(t *testing.T) {
	tests := []struct {
		shell string
		want  string
	}{
		{shell: "bash", want: "_raind_complete"},
		{shell: "zsh", want: "#compdef raind"},
		{shell: "fish", want: "complete -c raind"},
	}

	for _, tt := range tests {
		t.Run(tt.shell, func(t *testing.T) {
			app := NewApp()
			var out bytes.Buffer
			app.Writer = &out

			err := app.Run([]string{"raind", "completion", tt.shell})

			require.NoError(t, err)
			assert.Contains(t, out.String(), tt.want)
		})
	}
}

func TestCompletionRejectsUnsupportedShell(t *testing.T) {
	err := NewApp().Run([]string{"raind", "completion", "powershell"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported shell")
}
