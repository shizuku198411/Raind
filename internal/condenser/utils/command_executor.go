package utils

import (
	"io"
	"os"
	"os/exec"
	"strings"
)

func NewCommandFactory() *ExecCommandFactory {
	return &ExecCommandFactory{}
}

// commandFactory creates commandExecutor instances.
//
// The factory abstracts process creation so that callers do not depend
// directly on exec.Command. This makes the behavior testable by replacing
// the factory with a mock implementation.
type CommandFactory interface {
	Command(name string, args ...string) CommandExecutor
}

// execCommandFactory is the default implementation of commandFactory.
//
// It creates commandExecutor values backed by *exec.Cmd and launches
// real OS processes.
type ExecCommandFactory struct{}

// Command returns a commandExecutor that executes the given command
// using exec.Cmd.
func (e *ExecCommandFactory) Command(name string, args ...string) CommandExecutor {
	return &ExecCmd{cmd: exec.Command(name, args...)}
}

// commandExecutor represents a process that can be started.
//
// It provides a minimal surface over exec.Cmd so that command execution
// can be substituted or mocked in tests.
type CommandExecutor interface {
	Start() error
	Wait() error
	Run() error
	Output() ([]byte, error)
	CombineOutput() ([]byte, error)
	Pid() int
	SetEnv(envv []string)
	SetStdout(w io.Writer)
	SetStderr(w io.Writer)
	SetStdin(r io.Reader)
}

// execCmd is the concrete commandExecutor backed by exec.Cmd.
//
// It delegates all operations to the underlying exec.Cmd instance.
type ExecCmd struct {
	cmd *exec.Cmd
}

// Start starts the underlying process.
//
// It mirrors (*exec.Cmd).Start.
func (e *ExecCmd) Start() error {
	return e.cmd.Start()
}

func (e *ExecCmd) Wait() error {
	return e.cmd.Wait()
}

func (e *ExecCmd) Run() error {
	return e.cmd.Run()
}

func (e *ExecCmd) Output() ([]byte, error) {
	return e.cmd.Output()
}

func (e *ExecCmd) CombineOutput() ([]byte, error) {
	return e.cmd.CombinedOutput()
}

// Pid returns the PID of the started process.
//
// If the process has not been started, -1 is returned.
func (e *ExecCmd) Pid() int {
	if e.cmd.Process == nil {
		return -1
	}
	return e.cmd.Process.Pid
}

func (e *ExecCmd) SetEnv(envv []string) {
	base := e.cmd.Env
	if base == nil {
		base = os.Environ()
	}
	e.cmd.Env = mergeEnv(base, envv)
}

func mergeEnv(base, override []string) []string {
	out := append([]string(nil), base...)
	index := make(map[string]int, len(out))
	for i, kv := range out {
		if k, _, ok := strings.Cut(kv, "="); ok {
			index[k] = i
		}
	}
	for _, kv := range override {
		k, _, ok := strings.Cut(kv, "=")
		if !ok {
			out = append(out, kv)
			continue
		}
		if i, exists := index[k]; exists {
			out[i] = kv
		} else {
			index[k] = len(out)
			out = append(out, kv)
		}
	}
	return out
}

// SetStdout sets the stdout writer for the underlying command.
func (e *ExecCmd) SetStdout(w io.Writer) {
	e.cmd.Stdout = w
}

// SetStderr sets the stderr writer for the underlying command.
func (e *ExecCmd) SetStderr(w io.Writer) {
	e.cmd.Stderr = w
}

// SetStdin sets the standard input stream for the underlying command.
func (e *ExecCmd) SetStdin(r io.Reader) {
	e.cmd.Stdin = r
}
