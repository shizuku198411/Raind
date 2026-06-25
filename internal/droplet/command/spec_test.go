package command

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/status"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestParseCommandFlagUsesShellQuoting(t *testing.T) {
	// == exercise ==
	args, err := parseCommandFlag(`/bin/sh -c "echo hello world"`)

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, []string{"/bin/sh", "-c", "echo hello world"}, args)
}

func TestParseMountFlagDetectsDirectoryAndFileDefaults(t *testing.T) {
	// == setup ==
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	require.NoError(t, os.WriteFile(file, []byte("x"), 0644))

	// == exercise ==
	mounts, err := parseMountFlag([]string{
		dir + ":/mnt/dir",
		file + ":/mnt/file",
		dir + ":/mnt/explicit:ro,bind",
		dir + ":/mnt/readonly:ro",
		file + ":/mnt/file-readonly:ro",
	})

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, []spec.MountOption{
		{Source: dir, Destination: "/mnt/dir", Type: "", Options: []string{"bind"}},
		{Source: file, Destination: "/mnt/file", Type: "bind", Options: []string{"rbind", "rprivate"}},
		{Source: dir, Destination: "/mnt/explicit", Type: "", Options: []string{"ro", "bind"}},
		{Source: dir, Destination: "/mnt/readonly", Type: "", Options: []string{"bind", "ro"}},
		{Source: file, Destination: "/mnt/file-readonly", Type: "bind", Options: []string{"rbind", "rprivate", "ro"}},
	}, mounts)
}

func TestNamespaceFlagParsersAndMerge(t *testing.T) {
	// == exercise ==
	base, err := parseNamespaceFlag([]string{"mount", "network"})
	require.NoError(t, err)
	paths, err := parseNamespacePathFlag([]string{"network=/proc/1/ns/net", "uts=/proc/1/ns/uts"})
	require.NoError(t, err)
	merged := mergeNamespaceOptions(base, paths)

	// == assert ==
	assert.ElementsMatch(t, []spec.NamespaceOption{
		{Type: "mount"},
		{Type: "network", Path: "/proc/1/ns/net"},
		{Type: "uts", Path: "/proc/1/ns/uts"},
	}, merged)
	_, err = parseNamespaceFlag([]string{"bad"})
	require.Error(t, err)
	_, err = parseNamespacePathFlag([]string{"bad"})
	require.Error(t, err)
}

func TestParseHookFlagMapsCommandArgsAndEnv(t *testing.T) {
	// == exercise ==
	hooks, err := parseHookFlag(
		[]string{"/bin/hook,arg1,arg2", "/bin/other"},
		[]string{"A=1", "B=2"},
	)

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, []spec.HookOption{
		{Path: "/bin/hook", Args: []string{"arg1", "arg2"}, Env: []string{"A=1"}},
		{Path: "/bin/other", Env: []string{"B=2"}},
	}, hooks)
}

func TestParseHookFlagReturnsErrorWhenEnvHasNoMatchingHook(t *testing.T) {
	// == exercise ==
	_, err := parseHookFlag(nil, []string{"A=1"})

	// == assert ==
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no matching hook")
}

func TestNormalizeCapabilityFlag(t *testing.T) {
	// == exercise ==
	caps := normalizeCapabilityFlag([]string{" net_admin ", "CAP_NET_ADMIN", "sys_time", ""})

	// == assert ==
	assert.Equal(t, []string{"CAP_NET_ADMIN", "CAP_SYS_TIME"}, caps)
}

func TestCreateConfigOptionsBuildsConfigOptionsFromFlags(t *testing.T) {
	// == setup ==
	dir := t.TempDir()
	mountSrc := filepath.Join(dir, "mount")
	require.NoError(t, os.Mkdir(mountSrc, 0755))
	ctx := newSpecCLIContext(t, map[string]any{
		"rootfs":                  "/rootfs",
		"mount":                   []string{mountSrc + ":/data"},
		"cwd":                     "/work",
		"env":                     []string{"APP_ENV=test"},
		"cap-add":                 []string{"net_admin"},
		"cap-drop":                []string{"net_raw"},
		"security-profile":        spec.SecurityProfileDev,
		"command":                 `/bin/sh -c "echo hi"`,
		"ns":                      []string{"mount", "network"},
		"ns-path":                 []string{"network=/proc/1/ns/net"},
		"hostname":                "container-host",
		"host_if_name":            "veth-host",
		"bridge_if_name":          "raind0",
		"if_name":                 "eth0",
		"if_addr":                 "10.166.0.2/24",
		"if_gateway":              "10.166.0.1",
		"dns":                     []string{"1.1.1.1"},
		"image_layer":             []string{"/layers/a"},
		"upper_dir":               "/upper",
		"work_dir":                "/workdir",
		"hook-create-runtime":     []string{"/bin/hook"},
		"hook-create-runtime-env": []string{"A=1"},
	})

	// == exercise ==
	opts, err := createConfigOptions(ctx)

	// == assert ==
	require.NoError(t, err)
	assert.Equal(t, "/rootfs", opts.Rootfs)
	assert.Equal(t, []string{"/bin/sh", "-c", "echo hi"}, opts.Process.Args)
	assert.Equal(t, []string{"CAP_NET_ADMIN"}, opts.Process.CapAdd)
	assert.Equal(t, []string{"CAP_NET_RAW"}, opts.Process.CapDrop)
	assert.Equal(t, spec.SecurityProfileDev, opts.Security.ProfileName)
	assert.ElementsMatch(t, []spec.NamespaceOption{{Type: "mount"}, {Type: "network", Path: "/proc/1/ns/net"}}, opts.Namespace)
	assert.Equal(t, []spec.HookOption{{Path: "/bin/hook", Env: []string{"A=1"}}}, opts.Hooks.CreateRuntime)
}

func TestCreateConfigOptionsAcceptsResolvedSecurityOptions(t *testing.T) {
	ctx := newSpecCLIContext(t, map[string]any{
		"base-cap":          []string{"chown", "CAP_NET_RAW"},
		"seccomp-json":      `{"defaultAction":"SCMP_ACT_ALLOW"}`,
		"apparmor-profile":  "raind-default",
		"no-new-privileges": true,
	})

	opts, err := createConfigOptions(ctx)

	require.NoError(t, err)
	assert.Equal(t, []string{"CAP_CHOWN", "CAP_NET_RAW"}, opts.Security.BaseCapabilities)
	require.NotNil(t, opts.Security.Seccomp)
	assert.Equal(t, "SCMP_ACT_ALLOW", opts.Security.Seccomp.DefaultAction)
	assert.Equal(t, "raind-default", opts.Security.AppArmorProfile)
	assert.True(t, opts.Security.NoNewPrivileges)
}

func TestCreateConfigOptionsRejectsUnknownSecurityProfile(t *testing.T) {
	// == setup ==
	ctx := newSpecCLIContext(t, map[string]any{
		"security-profile": "unknown-profile",
	})

	// == exercise ==
	_, err := createConfigOptions(ctx)

	// == assert ==
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown security profile")
}

func TestRunCreateConfigFileWritesConfigJSON(t *testing.T) {
	// == setup ==
	outDir := t.TempDir()
	ctx := newSpecCLIContext(t, map[string]any{
		"output": outDir,
	})

	// == exercise ==
	err := runCreateConfigFile(ctx)

	// == assert ==
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(outDir, "config.json"))
}

func TestNewAppContainsExpectedCommands(t *testing.T) {
	// == exercise ==
	app := NewApp()

	// == assert ==
	var names []string
	for _, cmd := range app.Commands {
		names = append(names, cmd.Name)
	}
	assert.ElementsMatch(t, []string{
		"create", "start", "kill", "delete", "state", "run", "exec", "exec-shim", "spec", "list", "init", "shim", "attach",
	}, names)
}

func TestPrintListDefaultAndJSONFormats(t *testing.T) {
	// == setup ==
	list := []status.StatusObject{{Id: "c1", Status: "running", Pid: 123, Bundle: "/bundle"}}

	// == exercise/assert ==
	defaultOut := captureCommandStdout(t, func() { printList(list, "") })
	assert.Contains(t, defaultOut, "ID")
	assert.Contains(t, defaultOut, "c1")
	jsonOut := captureCommandStdout(t, func() { printList(list, "json") })
	assert.JSONEq(t, `[{"ociVersion":"","id":"c1","status":"running","exit_code":0,"reason":"","message":"","pid":123,"shimPid":0,"rootfs":"","bundle":"/bundle","annotations":{"io.raind.runtime.annotation.version":"","io.raind.net.config":"","io.raind.image.config":""}}]`, jsonOut)
}

func newSpecCLIContext(t *testing.T, values map[string]any) *cli.Context {
	t.Helper()
	set := flag.NewFlagSet("spec", flag.ContinueOnError)
	set.String("rootfs", "rootfs", "")
	set.Var(cli.NewStringSlice(), "mount", "")
	set.String("cwd", "/", "")
	set.Var(cli.NewStringSlice(), "env", "")
	set.Var(cli.NewStringSlice(), "cap-add", "")
	set.Var(cli.NewStringSlice(), "cap-drop", "")
	set.String("security-profile", spec.SecurityProfileDefault, "")
	set.Var(cli.NewStringSlice(), "base-cap", "")
	set.String("seccomp-json", "", "")
	set.String("apparmor-profile", "", "")
	set.Bool("no-new-privileges", false, "")
	set.String("command", "sh", "")
	set.Var(cli.NewStringSlice(), "ns", "")
	set.Var(cli.NewStringSlice(), "ns-path", "")
	set.String("hostname", "", "")
	set.String("host_if_name", "eth0", "")
	set.String("bridge_if_name", "raind_br0", "")
	set.String("if_name", "eth0", "")
	set.String("if_addr", "172.16.0.1/24", "")
	set.String("if_gateway", "172.16.0.254", "")
	set.Var(cli.NewStringSlice(), "dns", "")
	set.Var(cli.NewStringSlice(), "image_layer", "")
	set.String("upper_dir", "", "")
	set.String("work_dir", "", "")
	for _, name := range []string{
		"hook-prestart", "hook-prestart-env",
		"hook-create-runtime", "hook-create-runtime-env",
		"hook-create-container", "hook-create-container-env",
		"hook-start-container", "hook-start-container-env",
		"hook-poststart", "hook-poststart-env",
		"hook-stop-container", "hook-stop-container-env",
		"hook-poststop", "hook-poststop-env",
	} {
		set.Var(cli.NewStringSlice(), name, "")
	}
	set.String("output", ".", "")

	for name, value := range values {
		switch v := value.(type) {
		case string:
			require.NoError(t, set.Set(name, v))
		case []string:
			for _, item := range v {
				require.NoError(t, set.Set(name, item))
			}
		case bool:
			require.NoError(t, set.Set(name, fmt.Sprintf("%t", v)))
		}
	}
	return cli.NewContext(cli.NewApp(), set, nil)
}

func captureCommandStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	fn()
	require.NoError(t, w.Close())
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	require.NoError(t, err)
	return buf.String()
}
