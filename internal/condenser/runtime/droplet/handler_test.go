package droplet

import (
	"errors"
	"io"
	"testing"

	"raind/internal/condenser/runtime"
	"raind/internal/condenser/utils"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDropletHandlerSpecBuildsExpectedArgs(t *testing.T) {
	factory := &fakeCommandFactory{}
	handler := &DropletHandler{commandFactory: factory}

	err := handler.Spec(runtime.SpecModel{
		Rootfs:                 "/rootfs",
		Cwd:                    "/app",
		Command:                "/bin/sh -c test",
		Hostname:               "cid",
		HostInterface:          "eth0",
		BridgeInterface:        "raind0",
		ContainerInterface:     "rd_cid",
		ContainerInterfaceAddr: "10.166.0.1/24",
		ContainerGateway:       "10.166.0.254",
		UpperDir:               "/upper",
		WorkDir:                "/work",
		Output:                 "/out",
		Namespace:              []string{"mount", "pid"},
		NSPath:                 []string{"network=/proc/1/ns/net"},
		Env:                    []string{"A=B"},
		Mount:                  []string{"/host:/ctr:bind"},
		CapAdd:                 []string{"NET_ADMIN"},
		CapDrop:                []string{"MKNOD"},
		SecurityProfile:        "default",
		ContainerDns:           []string{"8.8.8.8"},
		ImageLayer:             []string{"/layer"},
		CreateRuntimeHook:      []string{"hook-runtime"},
		CreateRuntimeHookEnv:   []string{"ENV=runtime"},
		CreateContainerHook:    []string{"hook-container"},
		CreateContainerHookEnv: []string{"ENV=container"},
		StartContainerHook:     []string{"hook-start"},
		StartContainerHookEnv:  []string{"ENV=start"},
		PoststartHook:          []string{"hook-poststart"},
		PoststartHookEnv:       []string{"ENV=poststart"},
		StopContainerHook:      []string{"hook-stop"},
		StopContainerHookEnv:   []string{"ENV=stop"},
		PoststopHook:           []string{"hook-poststop"},
		PoststopHookEnv:        []string{"ENV=poststop"},
	})

	require.NoError(t, err)
	require.Len(t, factory.calls, 1)
	assert.Equal(t, "/usr/local/bin/droplet", factory.calls[0].name)
	assert.Equal(t, []string{
		"spec", "--rootfs", "/rootfs", "--cwd", "/app", "--command", "/bin/sh -c test",
		"--hostname", "cid", "--host_if_name", "eth0", "--bridge_if_name", "raind0",
		"--if_name", "rd_cid", "--if_addr", "10.166.0.1/24", "--if_gateway", "10.166.0.254",
		"--upper_dir", "/upper", "--work_dir", "/work", "--output", "/out", "--security-profile", "default",
		"--ns", "mount", "--ns", "pid", "--ns-path", "network=/proc/1/ns/net",
		"--env", "A=B", "--mount", "/host:/ctr:bind", "--cap-add", "NET_ADMIN",
		"--cap-drop", "MKNOD", "--dns", "8.8.8.8", "--image_layer", "/layer",
		"--hook-create-runtime", "hook-runtime", "--hook-create-runtime-env", "ENV=runtime",
		"--hook-create-container", "hook-container", "--hook-create-container-env", "ENV=container",
		"--hook-start-container", "hook-start", "--hook-start-container-env", "ENV=start",
		"--hook-poststart", "hook-poststart", "--hook-poststart-env", "ENV=poststart",
		"--hook-stop-container", "hook-stop", "--hook-stop-container-env", "ENV=stop",
		"--hook-poststop", "hook-poststop", "--hook-poststop-env", "ENV=poststop",
	}, factory.calls[0].args)
}

func TestDropletHandlerSpecUsesResolvedSecurityProfileArgs(t *testing.T) {
	factory := &fakeCommandFactory{}
	handler := &DropletHandler{commandFactory: factory}

	err := handler.Spec(runtime.SpecModel{
		Rootfs:          "/rootfs",
		Cwd:             "/",
		Command:         "sleep",
		Hostname:        "cid",
		HostInterface:   "eth0",
		BridgeInterface: "raind0",
		Output:          "/out",
		SecurityProfile: "dev",
		CapBase:         []string{"CAP_CHOWN", "CAP_NET_RAW"},
		SeccompJSON:     `{"defaultAction":"SCMP_ACT_ALLOW"}`,
		AppArmorProfile: "raind-default",
	})

	require.NoError(t, err)
	require.Len(t, factory.calls, 1)
	args := factory.calls[0].args
	assert.NotContains(t, args, "--security-profile")
	assert.Contains(t, args, "--base-cap")
	assert.Contains(t, args, "CAP_CHOWN")
	assert.Contains(t, args, "CAP_NET_RAW")
	assert.Contains(t, args, "--seccomp-json")
	assert.Contains(t, args, `{"defaultAction":"SCMP_ACT_ALLOW"}`)
	assert.Contains(t, args, "--apparmor-profile")
	assert.Contains(t, args, "raind-default")
}

func TestDropletHandlerCreateBuildsExpectedArgs(t *testing.T) {
	tests := []struct {
		name   string
		model  runtime.CreateModel
		podPid int
		want   commandCall
	}{
		{
			name:  "normal",
			model: runtime.CreateModel{ContainerId: "cid"},
			want:  commandCall{name: "/usr/local/bin/droplet", args: []string{"create", "cid"}},
		},
		{
			name:  "tty",
			model: runtime.CreateModel{ContainerId: "cid", Tty: true},
			want:  commandCall{name: "/usr/local/bin/droplet", args: []string{"create", "-t", "cid"}},
		},
		{
			name:   "pod nsenter",
			model:  runtime.CreateModel{ContainerId: "cid"},
			podPid: 123,
			want:   commandCall{name: "nsenter", args: []string{"-t", "123", "-U", "--", "/usr/local/bin/droplet", "create", "cid"}},
		},
		{
			name:   "pod nsenter tty",
			model:  runtime.CreateModel{ContainerId: "cid", Tty: true},
			podPid: 123,
			want:   commandCall{name: "nsenter", args: []string{"-t", "123", "-U", "--", "/usr/local/bin/droplet", "create", "-t", "cid"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := &fakeCommandFactory{}
			handler := &DropletHandler{commandFactory: factory}

			err := handler.Create(tt.model, tt.podPid)

			require.NoError(t, err)
			require.Len(t, factory.calls, 1)
			assert.Equal(t, tt.want, factory.calls[0])
		})
	}
}

func TestDropletHandlerLifecycleBuildsExpectedArgs(t *testing.T) {
	tests := []struct {
		name string
		run  func(*DropletHandler) error
		want commandCall
	}{
		{
			name: "start",
			run:  func(h *DropletHandler) error { return h.Start(runtime.StartModel{ContainerId: "cid"}) },
			want: commandCall{name: "/usr/local/bin/droplet", args: []string{"start", "cid"}},
		},
		{
			name: "stop",
			run:  func(h *DropletHandler) error { return h.Stop(runtime.StopModel{ContainerId: "cid"}) },
			want: commandCall{name: "/usr/local/bin/droplet", args: []string{"kill", "cid"}},
		},
		{
			name: "delete",
			run:  func(h *DropletHandler) error { return h.Delete(runtime.DeleteModel{ContainerId: "cid"}) },
			want: commandCall{name: "/usr/local/bin/droplet", args: []string{"delete", "cid"}},
		},
		{
			name: "exec",
			run: func(h *DropletHandler) error {
				return h.Exec(runtime.ExecModel{ContainerId: "cid", Entrypoint: []string{"/bin/echo", "ok"}})
			},
			want: commandCall{name: "/usr/local/bin/droplet", args: []string{"exec", "cid", "/bin/echo", "ok"}},
		},
		{
			name: "exec tty",
			run: func(h *DropletHandler) error {
				return h.Exec(runtime.ExecModel{ContainerId: "cid", Tty: true, Entrypoint: []string{"/bin/sh"}})
			},
			want: commandCall{name: "/usr/local/bin/droplet", args: []string{"exec", "-t", "cid", "/bin/sh"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory := &fakeCommandFactory{}
			handler := &DropletHandler{commandFactory: factory}

			err := tt.run(handler)

			require.NoError(t, err)
			require.Len(t, factory.calls, 1)
			assert.Equal(t, tt.want, factory.calls[0])
		})
	}
}

func TestDropletHandlerIncludesCommandOutputOnFailure(t *testing.T) {
	factory := &fakeCommandFactory{
		output: []byte("runtime stderr"),
		err:    errors.New("exit status 1"),
	}
	handler := &DropletHandler{commandFactory: factory}

	err := handler.Start(runtime.StartModel{ContainerId: "cid"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "droplet start failed: runtime stderr")
	assert.Contains(t, err.Error(), "exit status 1")
}

type commandCall struct {
	name string
	args []string
}

type fakeCommandFactory struct {
	calls  []commandCall
	output []byte
	err    error
}

func (f *fakeCommandFactory) Command(name string, args ...string) utils.CommandExecutor {
	f.calls = append(f.calls, commandCall{name: name, args: append([]string{}, args...)})
	return &fakeCommandExecutor{output: f.output, err: f.err}
}

type fakeCommandExecutor struct {
	output []byte
	err    error
}

func (e *fakeCommandExecutor) Start() error                   { return e.err }
func (e *fakeCommandExecutor) Wait() error                    { return e.err }
func (e *fakeCommandExecutor) Run() error                     { return e.err }
func (e *fakeCommandExecutor) Output() ([]byte, error)        { return e.output, e.err }
func (e *fakeCommandExecutor) CombineOutput() ([]byte, error) { return e.output, e.err }
func (e *fakeCommandExecutor) Pid() int                       { return 123 }
func (e *fakeCommandExecutor) SetEnv([]string)                {}
func (e *fakeCommandExecutor) SetStdout(io.Writer)            {}
func (e *fakeCommandExecutor) SetStderr(io.Writer)            {}
func (e *fakeCommandExecutor) SetStdin(io.Reader)             {}
