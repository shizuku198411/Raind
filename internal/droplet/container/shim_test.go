package container

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"raind/internal/droplet/spec"
	"raind/internal/droplet/utils"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

type fakeShimCommand struct {
	exitCode int
	sys      any
}

func (f *fakeShimCommand) Start() error                        { return nil }
func (f *fakeShimCommand) Wait() error                         { return nil }
func (f *fakeShimCommand) Run() error                          { return nil }
func (f *fakeShimCommand) Pid() int                            { return 0 }
func (f *fakeShimCommand) ExitCode() int                       { return f.exitCode }
func (f *fakeShimCommand) Sys() any                            { return f.sys }
func (f *fakeShimCommand) SetEnv([]string)                     {}
func (f *fakeShimCommand) SetStdout(io.Writer)                 {}
func (f *fakeShimCommand) SetStderr(io.Writer)                 {}
func (f *fakeShimCommand) SetStdin(io.Reader)                  {}
func (f *fakeShimCommand) SetSysProcAttr(*syscall.SysProcAttr) {}
func (f *fakeShimCommand) SetExtraFiles([]*os.File)            {}

func TestContainerShimWriteInitPidWritesPidFileAtomically(t *testing.T) {
	// == setup ==
	rootDir := t.TempDir()
	t.Setenv("RAIND_ROOT_DIR", rootDir)
	containerId := "container-1"
	shim := &ContainerShim{}

	// == exercise ==
	err := shim.writeInitPid(containerId, 1234)

	// == assert ==
	require.NoError(t, err)
	data, err := os.ReadFile(utils.InitPidFilePath(containerId))
	require.NoError(t, err)
	assert.Equal(t, "1234\n", string(data))
}

func TestContainerShimWriteInitPidRejectsInvalidPid(t *testing.T) {
	// == setup ==
	shim := &ContainerShim{}

	// == exercise ==
	err := shim.writeInitPid("container-1", 0)

	// == assert ==
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid init pid")
}

func TestContainerShimInitExitCodeUsesExitCodeWhenAvailable(t *testing.T) {
	// == setup ==
	shim := &ContainerShim{}

	// == exercise ==
	code := shim.InitExitCode(&fakeShimCommand{exitCode: 7})

	// == assert ==
	assert.Equal(t, 7, code)
}

func TestContainerShimInitExitCodeUsesSignalWaitStatus(t *testing.T) {
	// == setup ==
	shim := &ContainerShim{}

	// == exercise ==
	code := shim.InitExitCode(&fakeShimCommand{exitCode: -1, sys: syscall.WaitStatus(syscall.SIGKILL)})

	// == assert ==
	assert.Equal(t, 128+int(syscall.SIGKILL), code)
}

func TestShouldPrejoinRootlessPathNamespaces(t *testing.T) {
	containerSpec := spec.Spec{
		Annotations: spec.AnnotationObject{
			Rootless: `{"enabled":true}`,
		},
		LinuxSpec: spec.LinuxSpecObject{
			Namespaces: []spec.NamespaceObject{
				{Type: "network", Path: "/proc/1/ns/net"},
				{Type: "ipc", Path: "/proc/1/ns/ipc"},
				{Type: "uts", Path: "/proc/1/ns/uts"},
				{Type: "user"},
			},
		},
	}

	assert.True(t, shouldPrejoinRootlessPathNamespaces(containerSpec))
}

func TestShouldPrejoinRootlessPathNamespacesRequiresRootless(t *testing.T) {
	containerSpec := spec.Spec{
		LinuxSpec: spec.LinuxSpecObject{
			Namespaces: []spec.NamespaceObject{
				{Type: "network", Path: "/proc/1/ns/net"},
			},
		},
	}

	assert.False(t, shouldPrejoinRootlessPathNamespaces(containerSpec))
}

func TestContainerShimSetReasonAndMessageMapsExitCode(t *testing.T) {
	// == setup ==
	statusManager := &fakeDeleteStatusManager{}
	shim := &ContainerShim{containerStatusManager: statusManager}

	// == exercise ==
	require.NoError(t, shim.SetReasonAndMessage("container-1", 0, "", ""))
	require.NoError(t, shim.SetReasonAndMessage("container-1", 2, "", "failed"))

	// == assert ==
	assert.Equal(t, []deleteReasonUpdate{
		{containerId: "container-1", reason: "Completed", message: "exit code: 0"},
		{containerId: "container-1", reason: "Error", message: "failed"},
	}, statusManager.reasonUpdates)
}

func TestContainerShimReadFramesAndApplyWritesDataFrame(t *testing.T) {
	// == setup ==
	shim := &ContainerShim{}
	ptmx, err := os.CreateTemp("", "raind-shim-ptmx-*")
	require.NoError(t, err)
	defer ptmx.Close()
	var input bytes.Buffer
	input.Write(buildFrameForTest(frameData, []byte("hello")))

	// == exercise ==
	err = shim.readFramesAndApply(&input, ptmx)

	// == assert ==
	require.ErrorIs(t, err, io.EOF)
	data, err := os.ReadFile(ptmx.Name())
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestContainerShimReadFramesAndApplyRejectsTooLargeFrame(t *testing.T) {
	// == setup ==
	shim := &ContainerShim{}
	ptmx, err := os.CreateTemp("", "raind-shim-ptmx-*")
	require.NoError(t, err)
	defer ptmx.Close()
	var input bytes.Buffer
	header := make([]byte, 5)
	header[0] = frameData
	binary.BigEndian.PutUint32(header[1:5], 8*1024*1024+1)
	input.Write(header)

	// == exercise ==
	err = shim.readFramesAndApply(&input, ptmx)

	// == assert ==
	require.Error(t, err)
	assert.Contains(t, err.Error(), "frame too large")
}

func TestContainerShimHubAttachReplacesPreviousConnection(t *testing.T) {
	// == setup ==
	ptmx, err := os.CreateTemp("", "raind-shim-ptmx-*")
	require.NoError(t, err)
	defer ptmx.Close()
	h := newHub(ptmx, nil, nil)
	oldConnA, oldConnB := netPipeForTest(t)
	defer oldConnB.Close()
	newConnA, newConnB := netPipeForTest(t)
	defer newConnA.Close()
	defer newConnB.Close()

	// == exercise ==
	h.attach(oldConnA)
	h.attach(newConnA)

	// == assert ==
	assert.Equal(t, newConnA, h.conn)
	_ = oldConnB.SetWriteDeadline(time.Now().Add(10 * time.Millisecond))
	_, err = oldConnB.Write([]byte("x"))
	require.Error(t, err)
}

func TestSendConsoleFileDescriptorSendsReadableFD(t *testing.T) {
	// == setup ==
	socketPath := filepath.Join(t.TempDir(), "console.sock")
	ln, err := net.Listen("unix", socketPath)
	require.NoError(t, err)
	defer ln.Close()

	source, err := os.CreateTemp("", "raind-console-fd-*")
	require.NoError(t, err)
	defer source.Close()
	require.NoError(t, os.WriteFile(source.Name(), []byte("console"), 0644))
	_, err = source.Seek(0, io.SeekStart)
	require.NoError(t, err)

	fdCh := make(chan int, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()

		unixConn, ok := conn.(*net.UnixConn)
		if !ok {
			errCh <- errors.New("expected UnixConn")
			return
		}
		buf := make([]byte, 1)
		oob := make([]byte, unix.CmsgSpace(4))
		_, oobn, _, _, err := unixConn.ReadMsgUnix(buf, oob)
		if err != nil {
			errCh <- err
			return
		}
		msgs, err := unix.ParseSocketControlMessage(oob[:oobn])
		if err != nil {
			errCh <- err
			return
		}
		for _, msg := range msgs {
			fds, err := unix.ParseUnixRights(&msg)
			if err != nil {
				errCh <- err
				return
			}
			if len(fds) > 0 {
				fdCh <- fds[0]
				return
			}
		}
		errCh <- errors.New("no fd received")
	}()

	// == exercise ==
	err = sendConsoleFileDescriptor(socketPath, source)

	// == assert ==
	require.NoError(t, err)
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case fd := <-fdCh:
		received := os.NewFile(uintptr(fd), "received-console")
		defer received.Close()
		data, err := io.ReadAll(received)
		require.NoError(t, err)
		assert.Equal(t, "console", string(data))
	case <-time.After(time.Second):
		require.Fail(t, "timed out waiting for console fd")
	}
}

func TestConsoleWinsizeConvertsSpecConsoleSize(t *testing.T) {
	winsize, err := consoleWinsize(&spec.ConsoleSizeObject{Height: 24, Width: 80})

	require.NoError(t, err)
	require.NotNil(t, winsize)
	assert.Equal(t, uint16(24), winsize.Rows)
	assert.Equal(t, uint16(80), winsize.Cols)
}

func TestConsoleWinsizeRejectsInvalidConsoleSize(t *testing.T) {
	_, err := consoleWinsize(&spec.ConsoleSizeObject{Height: 0, Width: 80})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "process.consoleSize")

	_, err = consoleWinsize(&spec.ConsoleSizeObject{Height: 24, Width: spec.MaxConsoleSize + 1})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "process.consoleSize")
}

func buildFrameForTest(typ byte, payload []byte) []byte {
	frame := make([]byte, 5+len(payload))
	frame[0] = typ
	binary.BigEndian.PutUint32(frame[1:5], uint32(len(payload)))
	copy(frame[5:], payload)
	return frame
}

func buildTooLargeFrameForTest(typ byte) []byte {
	frame := make([]byte, 5)
	frame[0] = typ
	binary.BigEndian.PutUint32(frame[1:5], 8*1024*1024+1)
	return frame
}

func netPipeForTest(t *testing.T) (net.Conn, net.Conn) {
	t.Helper()
	a, b := net.Pipe()
	return a, b
}
