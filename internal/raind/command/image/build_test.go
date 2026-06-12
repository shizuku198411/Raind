package imagecommand

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestResolveBuildArgsUsesDockerStyleContextArgument(t *testing.T) {
	ctx := newBuildTestContext(t, []string{"-t", "local/app:latest", "."})

	contextDir, buildFile, legacy, err := resolveBuildArgs(ctx)

	require.NoError(t, err)
	assert.Equal(t, ".", contextDir)
	assert.Empty(t, buildFile)
	assert.False(t, legacy)
}

func TestResolveBuildArgsUsesFileAsDockerfilePath(t *testing.T) {
	ctx := newBuildTestContext(t, []string{"-t", "local/app:latest", "-f", "Dockerfile.prod", "."})

	contextDir, buildFile, legacy, err := resolveBuildArgs(ctx)

	require.NoError(t, err)
	assert.Equal(t, ".", contextDir)
	assert.Equal(t, "Dockerfile.prod", buildFile)
	assert.False(t, legacy)
}

func TestResolveBuildArgsSupportsLegacyFileAsContextDirectory(t *testing.T) {
	dir := t.TempDir()
	ctx := newBuildTestContext(t, []string{"-t", "local/app:latest", "-f", dir})

	contextDir, buildFile, legacy, err := resolveBuildArgs(ctx)

	require.NoError(t, err)
	assert.Equal(t, dir, contextDir)
	assert.Empty(t, buildFile)
	assert.True(t, legacy)
}

func TestResolveBuildArgsRejectsFileWithoutContext(t *testing.T) {
	ctx := newBuildTestContext(t, []string{"-t", "local/app:latest", "-f", "Dockerfile"})

	_, _, _, err := resolveBuildArgs(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing build context")
}

func newBuildTestContext(t *testing.T, args []string) *cli.Context {
	t.Helper()
	set := flag.NewFlagSet("build", flag.ContinueOnError)
	set.String("file", "", "")
	set.String("f", "", "")
	set.String("dockerfile", "", "")
	set.String("dripfile", "", "")
	set.String("tag", "", "")
	set.String("t", "", "")
	require.NoError(t, set.Parse(args))
	if set.Lookup("f").Value.String() != "" {
		require.NoError(t, set.Set("file", set.Lookup("f").Value.String()))
	}
	if set.Lookup("t").Value.String() != "" {
		require.NoError(t, set.Set("tag", set.Lookup("t").Value.String()))
	}
	return cli.NewContext(cli.NewApp(), set, nil)
}
