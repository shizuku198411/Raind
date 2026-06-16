package command

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestNewAppContainsExpectedTopLevelCommands(t *testing.T) {
	app := NewApp()
	names := map[string]bool{}
	for _, cmd := range app.Commands {
		names[cmd.Name] = true
	}

	for _, name := range []string{"container", "image", "network", "resource", "security", "bottle", "logs", "completion"} {
		assert.True(t, names[name], "missing command %s", name)
	}
	assert.False(t, names["policy"], "policy command should be nested under security")
}

func TestSecurityContainsPolicyAndProfileCommands(t *testing.T) {
	app := NewApp()
	var securityCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "security" {
			securityCmd = cmd
			break
		}
	}
	require.NotNil(t, securityCmd)

	names := map[string]bool{}
	for _, cmd := range securityCmd.Subcommands {
		names[cmd.Name] = true
	}

	assert.True(t, names["profile"], "missing security profile command")
	assert.True(t, names["policy"], "missing security policy command")
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
