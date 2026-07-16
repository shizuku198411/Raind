package command

import (
	"bytes"
	"testing"

	"raind/internal/raind/buildinfo"

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

func TestBottleContainsLifecycleWrapperCommands(t *testing.T) {
	app := NewApp()
	var bottleCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "bottle" {
			bottleCmd = cmd
			break
		}
	}
	require.NotNil(t, bottleCmd)

	names := map[string]bool{}
	for _, cmd := range bottleCmd.Subcommands {
		names[cmd.Name] = true
	}

	for _, name := range []string{"create", "start", "stop", "rm", "up", "down"} {
		assert.True(t, names[name], "missing bottle subcommand %s", name)
	}
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
		{shell: "bash", want: "raind __complete"},
		{shell: "zsh", want: "#compdef raind"},
		{shell: "fish", want: "__raind_complete"},
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

func TestCompleteSuggestsTopLevelCommands(t *testing.T) {
	app := NewApp()
	var out bytes.Buffer
	app.Writer = &out

	err := app.Run([]string{"raind", "__complete", ""})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "container\n")
	assert.Contains(t, out.String(), "resource\n")
	assert.Contains(t, out.String(), ":nofile\n")
}

func TestCompleteSuggestsResourceKinds(t *testing.T) {
	app := NewApp()
	var out bytes.Buffer
	app.Writer = &out

	err := app.Run([]string{"raind", "__complete", "resource", "get", ""})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "deployment\n")
	assert.Contains(t, out.String(), "pod\n")
	assert.Contains(t, out.String(), ":nofile\n")
}

func TestCompleteUsesFileCompletionForApply(t *testing.T) {
	app := NewApp()
	var out bytes.Buffer
	app.Writer = &out

	err := app.Run([]string{"raind", "__complete", "resource", "apply", "-f", ""})

	require.NoError(t, err)
	assert.Equal(t, ":default\n", out.String())
}

func TestCompleteSuggestsFlags(t *testing.T) {
	app := NewApp()
	var out bytes.Buffer
	app.Writer = &out

	err := app.Run([]string{"raind", "__complete", "resource", "get", "--n"})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "--namespace\n")
	assert.Contains(t, out.String(), ":nofile\n")
}

func TestCompletionRejectsUnsupportedShell(t *testing.T) {
	err := NewApp().Run([]string{"raind", "completion", "powershell"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported shell")
}

func TestVersionIncludesCommit(t *testing.T) {
	origVersion := buildinfo.Version
	origCommit := buildinfo.Commit
	t.Cleanup(func() {
		buildinfo.Version = origVersion
		buildinfo.Commit = origCommit
	})

	buildinfo.Version = "1.2.3"
	buildinfo.Commit = "abc1234"

	app := NewApp()
	var out bytes.Buffer
	app.Writer = &out

	err := app.Run([]string{"raind", "--version"})

	require.NoError(t, err)
	assert.Contains(t, out.String(), "raind version 1.2.3 (commit: abc1234)")
}
